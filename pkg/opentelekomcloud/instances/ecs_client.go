package instances

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"k8s.io/klog/v2"

	"github.com/opentelekomcloud/cloud-provider-opentelekomcloud/pkg/opentelekomcloud/config"
	"github.com/opentelekomcloud/cloud-provider-opentelekomcloud/pkg/opentelekomcloud/loadbalancer"
)

// ECSClient handles communication with the Swiss OTC ECS API
type ECSClient struct {
	endpoint  string
	projectID string
	signer    *loadbalancer.AKSKSigner
	client    *http.Client
}

// ServerDetail represents an ECS server from the API
type ServerDetail struct {
	ID               string                       `json:"id"`
	Name             string                       `json:"name"`
	Status           string                       `json:"status"`
	AvailabilityZone string                       `json:"OS-EXT-AZ:availability_zone"`
	Flavor           ServerFlavor                 `json:"flavor"`
	Addresses        map[string][]ServerAddress   `json:"addresses"`
	Metadata         map[string]string            `json:"metadata"`
}

// ServerFlavor represents the flavor of an ECS server
type ServerFlavor struct {
	ID string `json:"id"`
}

// ServerAddress represents a network address of an ECS server
type ServerAddress struct {
	Addr    string `json:"addr"`
	Version json.Number `json:"version"`
	Type    string `json:"OS-EXT-IPS:type"` // "fixed" or "floating"
}

// ListServersResponse is the API response for listing servers
type ListServersResponse struct {
	Servers []ServerDetail `json:"servers"`
}

// ShowServerResponse is the API response for getting a single server
type ShowServerResponse struct {
	Server ServerDetail `json:"server"`
}

// NewECSClient creates a new ECS API client
func NewECSClient(cfg *config.Config) *ECSClient {
	endpoints := cfg.GetEndpoints()
	return &ECSClient{
		endpoint:  endpoints.Compute,
		projectID: cfg.Auth.ProjectID,
		signer: &loadbalancer.AKSKSigner{
			AccessKey: cfg.Auth.AccessKey,
			SecretKey: cfg.Auth.SecretKey,
			ProjectID: cfg.Auth.ProjectID,
		},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// ListServers returns all ECS servers in the project
func (c *ECSClient) ListServers(ctx context.Context) ([]ServerDetail, error) {
	url := fmt.Sprintf("%s/v1/%s/cloudservers/detail", c.endpoint, c.projectID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.signer.SignRequest(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing servers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list servers failed: %d %s", resp.StatusCode, string(body))
	}

	var result ListServersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	klog.V(4).InfoS("Listed ECS servers", "count", len(result.Servers))
	return result.Servers, nil
}

// GetServerByID returns a single ECS server by ID
func (c *ECSClient) GetServerByID(ctx context.Context, serverID string) (*ServerDetail, error) {
	url := fmt.Sprintf("%s/v1/%s/cloudservers/%s", c.endpoint, c.projectID, serverID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.signer.SignRequest(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getting server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Server doesn't exist
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get server failed: %d %s", resp.StatusCode, string(body))
	}

	var result ShowServerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result.Server, nil
}

// FindServerByIP finds an ECS server by its internal IP address
func (c *ECSClient) FindServerByIP(ctx context.Context, ip string) (*ServerDetail, error) {
	servers, err := c.ListServers(ctx)
	if err != nil {
		return nil, err
	}

	for _, s := range servers {
		for _, addrs := range s.Addresses {
			for _, a := range addrs {
				if a.Addr == ip && a.Type == "fixed" {
					klog.V(4).InfoS("Found server by IP", "ip", ip, "server", s.Name, "id", s.ID)
					return &s, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no server found with IP %s", ip)
}
