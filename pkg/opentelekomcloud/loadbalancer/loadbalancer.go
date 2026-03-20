package loadbalancer

import (
	"context"
	"fmt"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/opentelekomcloud/cloud-provider-opentelekomcloud/pkg/opentelekomcloud/config"
)

// LoadBalancer implements the cloudprovider.LoadBalancer interface for Swiss OTC ELB v3
type LoadBalancer struct {
	config *config.Config
	client *ELBv3Client
}

// NewLoadBalancer creates a new LoadBalancer instance
func NewLoadBalancer(cfg *config.Config) (*LoadBalancer, error) {
	client, err := NewELBv3Client(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create ELB v3 client: %w", err)
	}

	return &LoadBalancer{
		config: cfg,
		client: client,
	}, nil
}

// GetLoadBalancer returns whether the specified load balancer exists, and if so, what its status is
// Implementations must treat the *v1.Service parameter as read-only and not modify it
func (lb *LoadBalancer) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	klog.V(2).InfoS("GetLoadBalancer called", "cluster", clusterName, "service", klog.KObj(service))

	lbName := lb.GetLoadBalancerName(ctx, clusterName, service)
	
	loadBalancer, err := lb.client.GetLoadBalancer(ctx, lbName)
	if err != nil {
		if IsNotFoundError(err) {
			klog.V(4).InfoS("LoadBalancer not found", "name", lbName)
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to get load balancer %s: %w", lbName, err)
	}

	status := &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP:       loadBalancer.VIPAddress,
			},
		},
	}

	klog.V(4).InfoS("LoadBalancer found", "name", lbName, "ip", loadBalancer.VIPAddress)
	return status, true, nil
}

// GetLoadBalancerName returns the name of the load balancer for the given service
func (lb *LoadBalancer) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	// Format: k8s-{cluster}-{namespace}-{service}
	// Swiss OTC ELB names have character limits, so we keep it concise
	name := fmt.Sprintf("k8s-%s-%s-%s", clusterName, service.Namespace, service.Name)
	
	// Swiss OTC ELB name restrictions: max 64 characters, alphanumeric + hyphens
	if len(name) > 64 {
		// Truncate but keep uniqueness with hash
		hash := generateShortHash(name)
		truncated := name[:55] // Leave room for hash
		name = fmt.Sprintf("%s-%s", truncated, hash)
	}
	
	return name
}

// EnsureLoadBalancer creates a new load balancer or updates the existing one
// Returns the status of the load balancer
func (lb *LoadBalancer) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	klog.V(2).InfoS("EnsureLoadBalancer called", "cluster", clusterName, "service", klog.KObj(service), "nodes", len(nodes))

	lbName := lb.GetLoadBalancerName(ctx, clusterName, service)

	// Check if load balancer already exists
	existingLB, err := lb.client.GetLoadBalancer(ctx, lbName)
	if err != nil && !IsNotFoundError(err) {
		return nil, fmt.Errorf("failed to check existing load balancer: %w", err)
	}

	var loadBalancer *LoadBalancerDetails
	if existingLB == nil {
		// Create new load balancer
		klog.V(2).InfoS("Creating new LoadBalancer", "name", lbName)
		
		guaranteed := true
		ipTargetEnable := true
		createReq := &CreateLoadBalancerRequest{
			Name:                 lbName,
			Description:          fmt.Sprintf("Kubernetes LoadBalancer for service %s/%s", service.Namespace, service.Name),
			VIPSubnetCIDRID:      lb.getSubnetCIDRID(service),
			ElbVirsubnetIDs:      []string{lb.getElbVirsubnetID(service)},
			AvailabilityZoneList: lb.getAvailabilityZones(),
			Guaranteed:           &guaranteed,
			IPTargetEnable:       &ipTargetEnable,
		}

		// Parse EIP bandwidth from annotation (0 = no EIP)
		eipBandwidth := 0
		if bwStr, ok := service.Annotations["otc.io/eip-bandwidth"]; ok {
			if bw, err := strconv.Atoi(bwStr); err == nil && bw > 0 {
				eipBandwidth = bw
				klog.V(2).InfoS("EIP requested via annotation", "bandwidth", bw)
			}
		}

		loadBalancer, err = lb.client.CreateLoadBalancer(ctx, createReq, eipBandwidth)
		if err != nil {
			return nil, fmt.Errorf("failed to create load balancer: %w", err)
		}
		
		klog.V(2).InfoS("LoadBalancer created", "name", lbName, "id", loadBalancer.ID, "eips", loadBalancer.EIPs)
	} else {
		loadBalancer = existingLB
		klog.V(4).InfoS("Using existing LoadBalancer", "name", lbName, "id", loadBalancer.ID)
	}

	// Configure listeners and pools for the service ports
	if err := lb.ensureListeners(ctx, loadBalancer, service, nodes); err != nil {
		return nil, fmt.Errorf("failed to configure listeners: %w", err)
	}

	// Configure security group on ELB VIP port to allow service traffic
	if loadBalancer.VIPPortID != "" {
		if err := lb.ensureELBSecurityGroup(ctx, loadBalancer.VIPPortID, service); err != nil {
			klog.V(2).InfoS("ELB security group configuration failed (non-fatal)", "error", err)
		}
	}


	// Use public IP (EIP) if available, otherwise use internal VIP
	ingressIP := loadBalancer.VIPAddress
	if len(loadBalancer.EIPs) > 0 && loadBalancer.EIPs[0].EIPAddress != "" {
		ingressIP = loadBalancer.EIPs[0].EIPAddress
		klog.V(2).InfoS("Using EIP as ingress address", "eip", ingressIP, "vip", loadBalancer.VIPAddress)
	} else if len(loadBalancer.PublicIPs) > 0 && loadBalancer.PublicIPs[0].PublicIPAddress != "" {
		ingressIP = loadBalancer.PublicIPs[0].PublicIPAddress
		klog.V(2).InfoS("Using public IP as ingress address", "publicip", ingressIP, "vip", loadBalancer.VIPAddress)
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: ingressIP,
			},
		},
	}, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer
func (lb *LoadBalancer) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.V(2).InfoS("UpdateLoadBalancer called", "cluster", clusterName, "service", klog.KObj(service), "nodes", len(nodes))

	lbName := lb.GetLoadBalancerName(ctx, clusterName, service)

	loadBalancer, err := lb.client.GetLoadBalancer(ctx, lbName)
	if err != nil {
		return fmt.Errorf("failed to get load balancer for update: %w", err)
	}

	// Sync listeners: add missing, remove orphaned
	if err := lb.syncListeners(ctx, loadBalancer, service, nodes); err != nil {
		return fmt.Errorf("failed to sync listeners: %w", err)
	}

	klog.V(2).InfoS("LoadBalancer updated", "name", lbName)
	return nil
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it exists
// Returns nil if the load balancer does not exist
func (lb *LoadBalancer) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.V(2).InfoS("EnsureLoadBalancerDeleted called", "cluster", clusterName, "service", klog.KObj(service))

	lbName := lb.GetLoadBalancerName(ctx, clusterName, service)
	
	loadBalancer, err := lb.client.GetLoadBalancer(ctx, lbName)
	if err != nil {
		if IsNotFoundError(err) {
			klog.V(4).InfoS("LoadBalancer already deleted", "name", lbName)
			return nil
		}
		return fmt.Errorf("failed to get load balancer for deletion: %w", err)
	}

	// Release EIP if one exists
	if loadBalancer.VIPPortID != "" {
		existingEIP, _ := lb.client.FindEIPByPortID(ctx, loadBalancer.VIPPortID)
		if existingEIP != nil {
			if err := lb.client.ReleaseEIP(ctx, existingEIP.ID); err != nil {
				klog.V(2).InfoS("EIP release failed (non-fatal)", "error", err)
			}
		}
	}

	// Must delete in order: listeners -> pools (members auto-deleted) -> ELB
	if err := lb.client.DeleteListeners(ctx, loadBalancer.ID); err != nil {
		return fmt.Errorf("failed to delete listeners for %s: %w", lbName, err)
	}

	if err := lb.client.DeletePools(ctx, loadBalancer.ID); err != nil {
		return fmt.Errorf("failed to delete pools for %s: %w", lbName, err)
	}

	if err := lb.client.DeleteLoadBalancer(ctx, loadBalancer.ID); err != nil {
		return fmt.Errorf("failed to delete load balancer %s: %w", lbName, err)
	}

	klog.V(2).InfoS("LoadBalancer deleted", "name", lbName, "id", loadBalancer.ID)

	// Release any EIPs that were bound to the ELB
	// OTC unbinds them automatically on ELB delete (status→DOWN) but does NOT release them,
	// leaving orphaned EIPs that cost money.
	for _, eip := range loadBalancer.EIPs {
		if eip.EIPID != "" {
			klog.V(2).InfoS("Releasing orphaned EIP", "eip", eip.EIPAddress, "id", eip.EIPID)
			if err := lb.client.ReleaseEIP(ctx, eip.EIPID); err != nil {
				klog.V(2).InfoS("Failed to release EIP (non-fatal)", "eip", eip.EIPAddress, "error", err)
			}
		}
	}
	// Also check publicips field (some OTC API versions use this instead of eips)
	for _, pip := range loadBalancer.PublicIPs {
		if pip.PublicIPID != "" {
			// Avoid double-release if already covered by EIPs
			alreadyReleased := false
			for _, eip := range loadBalancer.EIPs {
				if eip.EIPID == pip.PublicIPID {
					alreadyReleased = true
					break
				}
			}
			if !alreadyReleased {
				klog.V(2).InfoS("Releasing orphaned PublicIP", "ip", pip.PublicIPAddress, "id", pip.PublicIPID)
				if err := lb.client.ReleaseEIP(ctx, pip.PublicIPID); err != nil {
					klog.V(2).InfoS("Failed to release PublicIP (non-fatal)", "ip", pip.PublicIPAddress, "error", err)
				}
			}
		}
	}

	return nil
}

// getSubnetID extracts subnet ID from service annotations or uses default
// getEIPBandwidth returns the requested EIP bandwidth from annotation (0 = no EIP)
func (lb *LoadBalancer) getEIPBandwidth(service *v1.Service) int {
	if val, exists := service.Annotations["otc.io/eip-bandwidth"]; exists {
		bw := 0
		fmt.Sscanf(val, "%d", &bw)
		return bw
	}
	return 0
}

// ensureELBSecurityGroup configures the ELB VIP port's security group
// to allow incoming traffic on the service ports
func (lb *LoadBalancer) ensureELBSecurityGroup(ctx context.Context, vipPortID string, service *v1.Service) error {
	port, err := lb.client.GetPort(ctx, vipPortID)
	if err != nil {
		return fmt.Errorf("failed to get VIP port: %w", err)
	}

	if len(port.SecurityGroups) == 0 {
		klog.V(2).InfoS("VIP port has no security groups, skipping")
		return nil
	}

	sgID := port.SecurityGroups[0]
	klog.V(2).InfoS("Configuring ELB security group", "sg", sgID)

	// Determine allowed CIDR from annotation or default to 0.0.0.0/0
	allowedCIDR := "0.0.0.0/0"
	if cidr, exists := service.Annotations["otc.io/allowed-cidrs"]; exists && cidr != "" {
		allowedCIDR = cidr
	}

	// Add ingress rules for each service port
	for _, svcPort := range service.Spec.Ports {
		protocol := "tcp"
		if svcPort.Protocol == "UDP" {
			protocol = "udp"
		}
		if err := lb.client.EnsureSecurityGroupRule(ctx, sgID, protocol, int(svcPort.Port), allowedCIDR); err != nil {
			klog.V(2).InfoS("Failed to add SG rule", "port", svcPort.Port, "error", err)
		}
	}

	return nil
}

func (lb *LoadBalancer) getSubnetCIDRID(service *v1.Service) string {
	// Check for annotation override
	if id, exists := service.Annotations["otc.io/subnet-cidr-id"]; exists {
		return id
	}
	// Fallback to legacy annotation
	if id, exists := service.Annotations["otc.io/subnet-id"]; exists {
		return id
	}
	// Use cloud-config default
	return lb.config.Network.SubnetID
}

func (lb *LoadBalancer) getElbVirsubnetID(service *v1.Service) string {
	// Check for annotation override
	if id, exists := service.Annotations["otc.io/elb-virsubnet-id"]; exists {
		return id
	}
	// Use cloud-config default
	if lb.config.Network.NetworkID != "" {
		return lb.config.Network.NetworkID
	}
	// Fallback to subnet_id (works on Swiss OTC where both are the same)
	return lb.config.Network.SubnetID
}

// generateShortHash creates a short hash for name truncation
func generateShortHash(input string) string {
	// Simple hash for demonstration - in production use crypto/sha256
	hash := 0
	for _, c := range input {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return fmt.Sprintf("%x", hash%0xFFFF) // 4 hex characters
}

// ensureListeners configures ELB listeners, pools, members and health monitors for service ports
func (lb *LoadBalancer) ensureListeners(ctx context.Context, loadBalancer *LoadBalancerDetails, service *v1.Service, nodes []*v1.Node) error {
	klog.V(2).InfoS("Configuring listeners and pools", "lb", loadBalancer.ID, "ports", len(service.Spec.Ports))

	for _, port := range service.Spec.Ports {
		protocol := "TCP"
		if port.Protocol == "UDP" {
			protocol = "UDP"
		}

		poolName := fmt.Sprintf("%s-pool-%d", loadBalancer.Name, port.Port)
		listenerName := fmt.Sprintf("%s-listener-%d", loadBalancer.Name, port.Port)

		// 1. Create backend pool
		pool, err := lb.client.EnsurePool(ctx, loadBalancer.ID, poolName, protocol)
		if err != nil {
			return fmt.Errorf("failed to ensure pool for port %d: %w", port.Port, err)
		}
		klog.V(2).InfoS("Pool ensured", "pool", pool.ID, "name", poolName)

		// 2. Add/update members (node IPs with NodePort)
		if err := lb.client.EnsureMembers(ctx, pool.ID, nodes, port.NodePort); err != nil {
			return fmt.Errorf("failed to ensure members for pool %s: %w", pool.ID, err)
		}
		klog.V(2).InfoS("Members ensured", "pool", pool.ID, "nodePort", port.NodePort)

		// 3. Create listener
		if err := lb.client.EnsureListener(ctx, loadBalancer.ID, pool.ID, listenerName, protocol, int(port.Port)); err != nil {
			return fmt.Errorf("failed to ensure listener for port %d: %w", port.Port, err)
		}
		klog.V(2).InfoS("Listener ensured", "name", listenerName, "port", port.Port)

		// 4. Create health monitor (configurable via annotations)
		hmConfig := lb.getHealthMonitorConfig(service, protocol)
		if err := lb.client.EnsureHealthMonitorWithConfig(ctx, pool.ID, hmConfig); err != nil {
			klog.V(2).InfoS("Health monitor setup failed (non-fatal)", "error", err)
		}
	}

	return nil
}

// updateBackendPools updates the backend pool members
func (lb *LoadBalancer) updateBackendPools(ctx context.Context, loadBalancer *LoadBalancerDetails, service *v1.Service, nodes []*v1.Node) error {
	klog.V(2).InfoS("Updating backend pools", "lb", loadBalancer.ID, "nodes", len(nodes))
	return lb.syncListeners(ctx, loadBalancer, service, nodes)
}

// syncListeners performs a diff-based sync between desired (Service ports) and
// actual (ELB listeners). It adds missing listeners/pools, updates members,
// and removes orphaned listeners/pools for ports no longer in the Service.
func (lb *LoadBalancer) syncListeners(ctx context.Context, loadBalancer *LoadBalancerDetails, service *v1.Service, nodes []*v1.Node) error {
	klog.V(2).InfoS("Syncing listeners", "lb", loadBalancer.ID, "desiredPorts", len(service.Spec.Ports))

	// Step 1: List existing listeners
	existingListeners, err := lb.client.ListListeners(ctx, loadBalancer.ID)
	if err != nil {
		klog.V(2).InfoS("Could not list listeners, falling back to ensure-only", "error", err)
		return lb.ensureListeners(ctx, loadBalancer, service, nodes)
	}

	// Build set of desired listener names
	desiredNames := make(map[string]bool)
	for _, port := range service.Spec.Ports {
		listenerName := fmt.Sprintf("%s-listener-%d", loadBalancer.Name, port.Port)
		desiredNames[listenerName] = true
	}

	// Step 2: Delete orphaned listeners (not in desired set)
	for _, listener := range existingListeners {
		if !desiredNames[listener.Name] {
			klog.V(2).InfoS("Removing orphaned listener", "name", listener.Name, "id", listener.ID, "port", listener.ProtocolPort)

			// Delete the listener first
			if err := lb.client.DeleteListener(ctx, listener.ID); err != nil {
				klog.V(2).InfoS("Failed to delete orphaned listener", "id", listener.ID, "error", err)
			}

			// Delete the associated pool
			if listener.DefaultPoolID != "" {
				if err := lb.client.DeletePool(ctx, listener.DefaultPoolID); err != nil {
					klog.V(2).InfoS("Failed to delete orphaned pool", "id", listener.DefaultPoolID, "error", err)
				}
			}
		}
	}

	// Step 3: Ensure all desired listeners exist with correct members
	return lb.ensureListeners(ctx, loadBalancer, service, nodes)
}
// getHealthMonitorConfig builds a HealthMonitorConfig from service annotations.
// Supported annotations:
//   - otc.io/health-check-type: TCP (default), HTTP, HTTPS
//   - otc.io/health-check-delay: interval in seconds (default: 5)
//   - otc.io/health-check-timeout: timeout in seconds (default: 10)
//   - otc.io/health-check-max-retries: max retries (default: 3)
//   - otc.io/health-check-url-path: URL path for HTTP/HTTPS (default: "/")
//   - otc.io/health-check-http-method: HTTP method (default: "GET")
func (lb *LoadBalancer) getHealthMonitorConfig(service *v1.Service, protocol string) *HealthMonitorConfig {
	cfg := DefaultHealthMonitorConfig(protocol)

	if service.Annotations == nil {
		return cfg
	}

	if val, ok := service.Annotations["otc.io/health-check-type"]; ok {
		switch val {
		case "TCP", "HTTP", "HTTPS", "UDP_CONNECT":
			cfg.Type = val
		default:
			klog.V(2).InfoS("Ignoring invalid health check type", "value", val)
		}
	}

	if val, ok := service.Annotations["otc.io/health-check-delay"]; ok {
		if v, err := strconv.Atoi(val); err == nil && v >= 1 && v <= 50 {
			cfg.Delay = v
		}
	}

	if val, ok := service.Annotations["otc.io/health-check-timeout"]; ok {
		if v, err := strconv.Atoi(val); err == nil && v >= 1 && v <= 50 {
			cfg.Timeout = v
		}
	}

	if val, ok := service.Annotations["otc.io/health-check-max-retries"]; ok {
		if v, err := strconv.Atoi(val); err == nil && v >= 1 && v <= 10 {
			cfg.MaxRetries = v
		}
	}

	if val, ok := service.Annotations["otc.io/health-check-url-path"]; ok && val != "" {
		cfg.URLPath = val
	}

	if val, ok := service.Annotations["otc.io/health-check-http-method"]; ok {
		switch val {
		case "GET", "HEAD", "POST":
			cfg.HTTPMethod = val
		}
	}

	return cfg
}

func (lb *LoadBalancer) getAvailabilityZones() []string {
	if len(lb.config.LoadBalancer.AvailabilityZones) > 0 {
		return lb.config.LoadBalancer.AvailabilityZones
	}
	return []string{"eu-ch2a", "eu-ch2b"}
}
