package instances

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	"github.com/opentelekomcloud/cloud-provider-opentelekomcloud/pkg/opentelekomcloud/config"
)

// InstancesV2 implements the cloudprovider.InstancesV2 interface for Swiss OTC
type InstancesV2 struct {
	config    *config.Config
	ecsClient *ECSClient

	// Cache to avoid excessive API calls
	cacheMu    sync.RWMutex
	cache      map[string]*ServerDetail // keyed by internal IP
	cacheTime  time.Time
	cacheTTL   time.Duration
}

// NewInstancesV2 creates a new InstancesV2 controller
func NewInstancesV2(cfg *config.Config) (*InstancesV2, error) {
	return &InstancesV2{
		config:    cfg,
		ecsClient: NewECSClient(cfg),
		cache:     make(map[string]*ServerDetail),
		cacheTTL:  2 * time.Minute,
	}, nil
}

// refreshCache fetches all servers and populates the IP->server cache
func (i *InstancesV2) refreshCache(ctx context.Context) error {
	i.cacheMu.Lock()
	defer i.cacheMu.Unlock()

	// Skip if cache is fresh
	if time.Since(i.cacheTime) < i.cacheTTL {
		return nil
	}

	servers, err := i.ecsClient.ListServers(ctx)
	if err != nil {
		return fmt.Errorf("refreshing server cache: %w", err)
	}

	newCache := make(map[string]*ServerDetail)
	for idx := range servers {
		s := &servers[idx]
		for _, addrs := range s.Addresses {
			for _, a := range addrs {
				if a.Type == "fixed" && a.Version.String() == "4" {
					newCache[a.Addr] = s
				}
			}
		}
	}

	i.cache = newCache
	i.cacheTime = time.Now()
	klog.V(2).InfoS("Server cache refreshed", "servers", len(servers), "mappings", len(newCache))
	return nil
}

// getServerForNode finds the ECS server corresponding to a Kubernetes node
func (i *InstancesV2) getServerForNode(ctx context.Context, node *v1.Node) (*ServerDetail, error) {
	// Try to find by provider ID first (if already set)
	if node.Spec.ProviderID != "" {
		serverID := extractServerID(node.Spec.ProviderID)
		if serverID != "" {
			server, err := i.ecsClient.GetServerByID(ctx, serverID)
			if err != nil {
				klog.V(2).InfoS("Failed to get server by provider ID, falling back to IP", "providerID", node.Spec.ProviderID, "error", err)
			} else if server != nil {
				return server, nil
			}
		}
	}

	// Fall back to finding by internal IP
	if err := i.refreshCache(ctx); err != nil {
		return nil, err
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			i.cacheMu.RLock()
			server, found := i.cache[addr.Address]
			i.cacheMu.RUnlock()
			if found {
				return server, nil
			}
		}
	}

	return nil, fmt.Errorf("no ECS server found for node %s", node.Name)
}

// extractServerID extracts the server UUID from a provider ID like "opentelekomcloud:///eu-ch2a/uuid"
func extractServerID(providerID string) string {
	// Format: opentelekomcloud:///<az>/<server-id>
	parts := splitProviderID(providerID)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func splitProviderID(providerID string) []string {
	// Handle various formats:
	// opentelekomcloud:///eu-ch2a/uuid
	// otc://node-name (legacy)
	trimmed := providerID
	for _, prefix := range []string{"opentelekomcloud:///", "otc://"} {
		if len(trimmed) > len(prefix) && trimmed[:len(prefix)] == prefix {
			trimmed = trimmed[len(prefix):]
			break
		}
	}
	var parts []string
	current := ""
	for _, c := range trimmed {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// InstanceExists returns true if the instance for the given node exists according to the cloud provider
func (i *InstancesV2) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	server, err := i.getServerForNode(ctx, node)
	if err != nil {
		klog.V(2).InfoS("InstanceExists: server not found, assuming exists", "node", node.Name, "error", err)
		// Return true to avoid Kubernetes deleting the node on transient errors
		return true, nil
	}

	exists := server.Status != "DELETED" && server.Status != "SOFT_DELETED"
	klog.V(4).InfoS("InstanceExists", "node", node.Name, "server", server.Name, "status", server.Status, "exists", exists)
	return exists, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider
func (i *InstancesV2) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	server, err := i.getServerForNode(ctx, node)
	if err != nil {
		klog.V(2).InfoS("InstanceShutdown: server not found", "node", node.Name, "error", err)
		return false, nil
	}

	shutdown := server.Status == "SHUTOFF" || server.Status == "STOPPED"
	klog.V(4).InfoS("InstanceShutdown", "node", node.Name, "status", server.Status, "shutdown", shutdown)
	return shutdown, nil
}

// InstanceMetadata returns the instance's metadata
func (i *InstancesV2) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	server, err := i.getServerForNode(ctx, node)
	if err != nil {
		klog.InfoS("InstanceMetadata: could not find server, returning basic metadata", "node", node.Name, "error", err)
		// Return basic metadata so the node isn't rejected
		return &cloudprovider.InstanceMetadata{
			ProviderID:    "opentelekomcloud:///" + i.config.Region + "/" + node.Name,
			InstanceType:  "unknown",
			NodeAddresses: nodeAddressesFromNode(node),
			Zone:          i.config.Region,
			Region:        i.config.Region,
		}, nil
	}

	// Build node addresses from ECS data
	nodeAddresses := []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: node.Name},
	}
	for _, addrs := range server.Addresses {
		for _, a := range addrs {
			if a.Version.String() != "4" {
				continue
			}
			if a.Type == "fixed" {
				nodeAddresses = append(nodeAddresses, v1.NodeAddress{
					Type:    v1.NodeInternalIP,
					Address: a.Addr,
				})
			} else if a.Type == "floating" {
				nodeAddresses = append(nodeAddresses, v1.NodeAddress{
					Type:    v1.NodeExternalIP,
					Address: a.Addr,
				})
			}
		}
	}

	providerID := fmt.Sprintf("opentelekomcloud:///%s/%s", server.AvailabilityZone, server.ID)

	metadata := &cloudprovider.InstanceMetadata{
		ProviderID:    providerID,
		InstanceType:  server.Flavor.ID,
		NodeAddresses: nodeAddresses,
		Zone:          server.AvailabilityZone,
		Region:        i.config.Region,
	}

	klog.InfoS("InstanceMetadata",
		"node", node.Name,
		"providerID", providerID,
		"instanceType", server.Flavor.ID,
		"zone", server.AvailabilityZone,
	)

	return metadata, nil
}

// nodeAddressesFromNode extracts addresses from the node's existing status
func nodeAddressesFromNode(node *v1.Node) []v1.NodeAddress {
	addrs := []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: node.Name},
	}
	for _, a := range node.Status.Addresses {
		if a.Type == v1.NodeInternalIP {
			addrs = append(addrs, a)
		}
	}
	return addrs
}
