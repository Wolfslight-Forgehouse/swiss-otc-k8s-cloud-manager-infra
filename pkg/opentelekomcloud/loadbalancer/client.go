package loadbalancer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/opentelekomcloud/cloud-provider-opentelekomcloud/pkg/opentelekomcloud/config"
)

// ELBv3Client handles Swiss OTC ELB v3 API calls
type ELBv3Client struct {
	config      *config.Config
	endpoints   *config.Endpoints
	httpClient  *http.Client
	authToken   string
	tokenExpiry time.Time
	akskSigner  *AKSKSigner
}

// LoadBalancerDetails represents an ELB load balancer
type LoadBalancerDetails struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	VIPAddress        string    `json:"vip_address"`
	VIPPortID         string    `json:"vip_port_id"`
	VIPSubnetCIDRID   string    `json:"vip_subnet_cidr_id"`
	Provider          string    `json:"provider"`
	OperatingStatus   string    `json:"operating_status"`
	ProvisioningStatus string        `json:"provisioning_status"`
	PublicIPs          []PublicIPInfo `json:"publicips,omitempty"`
	EIPs               []EIPInfo      `json:"eips,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

// PublicIPInfo represents a public IP associated with an ELB
type PublicIPInfo struct {
	PublicIPID      string `json:"publicip_id"`
	PublicIPAddress string `json:"publicip_address"`
	IPVersion       int    `json:"ip_version"`
}

// EIPInfo represents an Elastic IP associated with an ELB
type EIPInfo struct {
	EIPID      string `json:"eip_id"`
	EIPAddress string `json:"eip_address"`
	IPVersion  int    `json:"ip_version"`
}

// EIPRequest represents the publicip block for ELB creation with automatic EIP
type EIPRequest struct {
	IPVersion   int              `json:"ip_version"`
	NetworkType string           `json:"network_type"`
	Bandwidth   BandwidthRequest `json:"bandwidth"`
}

// BandwidthRequest represents bandwidth settings for EIP creation
type BandwidthRequest struct {
	Size       int    `json:"size"`
	ShareType  string `json:"share_type"`
	ChargeMode string `json:"charge_mode"`
	Name       string `json:"name"`
}

// CreateLoadBalancerRequest represents a request to create an ELB
type CreateLoadBalancerRequest struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description,omitempty"`
	VIPSubnetCIDRID      string   `json:"vip_subnet_cidr_id"`
	ElbVirsubnetIDs      []string `json:"elb_virsubnet_ids,omitempty"`
	AvailabilityZoneList []string `json:"availability_zone_list"`
	Provider             string   `json:"provider,omitempty"`
	VpcID                string   `json:"vpc_id,omitempty"`
	Guaranteed           *bool    `json:"guaranteed,omitempty"`
	IPTargetEnable       *bool    `json:"ip_target_enable,omitempty"`
}

// AuthResponse represents the response from identity authentication
type AuthResponse struct {
	Token struct {
		ExpiresAt time.Time `json:"expires_at"`
	} `json:"token"`
}

// NotFoundError represents a resource not found error
type NotFoundError struct {
	Message string
}

func (e NotFoundError) Error() string {
	return e.Message
}

// IsNotFoundError checks if an error is a not found error
func IsNotFoundError(err error) bool {
	_, ok := err.(NotFoundError)
	return ok
}

// NewELBv3Client creates a new ELB v3 client for Swiss OTC
func NewELBv3Client(cfg *config.Config) (*ELBv3Client, error) {
	client := &ELBv3Client{
		config:    cfg,
		endpoints: cfg.GetEndpoints(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Initialize AK/SK signer if credentials are provided
	if cfg.Auth.AccessKey != "" && cfg.Auth.SecretKey != "" {
		client.akskSigner = &AKSKSigner{
			AccessKey: cfg.Auth.AccessKey,
			SecretKey: cfg.Auth.SecretKey,
			ProjectID: cfg.Auth.ProjectID,
		}
		klog.V(2).InfoS("Using AK/SK authentication for ELB API")
	} else {
		klog.V(2).InfoS("Using password authentication for ELB API")
	}

	return client, nil
}

// authenticate obtains an auth token from Swiss OTC IAM
func (c *ELBv3Client) authenticate(ctx context.Context) error {
	klog.V(4).InfoS("Authenticating with Swiss OTC IAM", "url", c.endpoints.Identity)

	authReq := map[string]interface{}{
		"auth": map[string]interface{}{
			"identity": map[string]interface{}{
				"methods": []string{"password"},
				"password": map[string]interface{}{
					"user": map[string]interface{}{
						"name":     c.config.Auth.Username,
						"password": c.config.Auth.Password,
						"domain": map[string]interface{}{
							"name": c.config.Auth.UserDomainName,
						},
					},
				},
			},
			"scope": map[string]interface{}{
				"project": map[string]interface{}{
					"id": c.config.Auth.ProjectID,
				},
			},
		},
	}

	reqBody, err := json.Marshal(authReq)
	if err != nil {
		return fmt.Errorf("failed to marshal auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoints.Identity+"/auth/tokens", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Get token from X-Subject-Token header
	c.authToken = resp.Header.Get("X-Subject-Token")
	if c.authToken == "" {
		return fmt.Errorf("no auth token received in response")
	}

	// Parse token expiry from response body
	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		klog.V(4).InfoS("Warning: could not parse token expiry", "error", err)
		// Set expiry to 1 hour from now as fallback
		c.tokenExpiry = time.Now().Add(time.Hour)
	} else {
		c.tokenExpiry = authResp.Token.ExpiresAt
	}

	klog.V(4).InfoS("Authentication successful", "expiry", c.tokenExpiry)
	return nil
}

// ensureAuthenticated checks if the token is valid and re-authenticates if needed
func (c *ELBv3Client) ensureAuthenticated(ctx context.Context) error {
	if time.Now().Add(5 * time.Minute).After(c.tokenExpiry) {
		klog.V(4).InfoS("Token expired or expiring soon, re-authenticating")
		return c.authenticate(ctx)
	}
	return nil
}

// signOrTokenAuth signs a request with AK/SK or sets the auth token
func (c *ELBv3Client) signOrTokenAuth(ctx context.Context, req *http.Request) error {
	if c.akskSigner != nil {
		return c.akskSigner.SignRequest(req)
	}
	if err := c.ensureAuthenticated(ctx); err != nil {
		return err
	}
	req.Header.Set("X-Auth-Token", c.authToken)
	return nil
}

// makeRequest makes an authenticated request to Swiss OTC API
func (c *ELBv3Client) makeRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	// Swiss OTC ELB v3 requires project_id in the path: /v3/{project_id}/...
	// Transform /v3/elb/... to /v3/{project_id}/elb/...
	apiPath := path
	if strings.HasPrefix(path, "/v3/elb/") {
		apiPath = fmt.Sprintf("/v3/%s/elb/%s", c.config.Auth.ProjectID, strings.TrimPrefix(path, "/v3/elb/"))
	}
	url := c.endpoints.ELB + apiPath
	
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if err := c.signOrTokenAuth(ctx, req); err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}

	klog.V(6).InfoS("Making API request", "method", method, "url", url)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// GetLoadBalancer retrieves a load balancer by name
func (c *ELBv3Client) GetLoadBalancer(ctx context.Context, name string) (*LoadBalancerDetails, error) {
	// Swiss OTC ELB v3 API: GET /v3/elb/loadbalancers?name={name}
	resp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/v3/elb/loadbalancers?name=%s", name), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, NotFoundError{Message: fmt.Sprintf("load balancer %s not found", name)}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get load balancer: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		LoadBalancers []LoadBalancerDetails `json:"loadbalancers"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.LoadBalancers) == 0 {
		return nil, NotFoundError{Message: fmt.Sprintf("load balancer %s not found", name)}
	}

	return &result.LoadBalancers[0], nil
}

// CreateLoadBalancer creates a new load balancer.
// If eipBandwidth > 0, an EIP is automatically created and bound to the ELB.
// NOTE: Swiss OTC eu-ch2 requires elb_virsubnet_ids to be set for EIP binding to work.
func (c *ELBv3Client) CreateLoadBalancer(ctx context.Context, req *CreateLoadBalancerRequest, eipBandwidth int) (*LoadBalancerDetails, error) {
	klog.V(2).InfoS("Creating load balancer", "name", req.Name, "eipBandwidth", eipBandwidth)

	// Swiss OTC ELB v3 API: POST /v3/elb/loadbalancers
	// IMPORTANT: On Swiss OTC eu-ch2, the publicip block MUST be nested INSIDE
	// the loadbalancer object (not at the top level). Additionally, elb_virsubnet_ids
	// must be set for EIP binding to work. Without either, EIP is silently ignored.
	lbBody := map[string]interface{}{
		"name":                  req.Name,
		"vip_subnet_cidr_id":    req.VIPSubnetCIDRID,
		"availability_zone_list": req.AvailabilityZoneList,
	}
	if req.Description != "" {
		lbBody["description"] = req.Description
	}
	if len(req.ElbVirsubnetIDs) > 0 {
		lbBody["elb_virsubnet_ids"] = req.ElbVirsubnetIDs
	}
	if req.Provider != "" {
		lbBody["provider"] = req.Provider
	}
	if req.VpcID != "" {
		lbBody["vpc_id"] = req.VpcID
	}
	if req.Guaranteed != nil {
		lbBody["guaranteed"] = *req.Guaranteed
	}
	if req.IPTargetEnable != nil {
		lbBody["ip_target_enable"] = *req.IPTargetEnable
	}

	// Add EIP (publicip) block INSIDE the loadbalancer object
	if eipBandwidth > 0 {
		if len(req.ElbVirsubnetIDs) == 0 {
			klog.Warning("EIP requested but elb_virsubnet_ids is empty — EIP binding will be silently ignored on Swiss OTC eu-ch2")
		}
		lbBody["publicip"] = map[string]interface{}{
			"ip_version":   4,
			"network_type": "5_bgp",
			"bandwidth": map[string]interface{}{
				"size":        eipBandwidth,
				"share_type":  "PER",
				"charge_mode": "traffic",
				"name":        fmt.Sprintf("%s-bw", req.Name),
			},
		}
		klog.V(2).InfoS("EIP requested at ELB creation time (inside loadbalancer body)", "bandwidth", eipBandwidth)
	}

	requestBody := map[string]interface{}{
		"loadbalancer": lbBody,
	}

	resp, err := c.makeRequest(ctx, "POST", "/v3/elb/loadbalancers", requestBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create load balancer: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		LoadBalancer LoadBalancerDetails `json:"loadbalancer"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	klog.V(2).InfoS("Load balancer created", "name", req.Name, "id", result.LoadBalancer.ID)
	return &result.LoadBalancer, nil
}

// DeleteLoadBalancer deletes a load balancer by ID.
// Retries on 409 Conflict (e.g. listeners still being deleted) with exponential backoff.
func (c *ELBv3Client) DeleteLoadBalancer(ctx context.Context, id string) error {
	klog.V(2).InfoS("Deleting load balancer", "id", id)

	maxRetries := 5
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := c.makeRequest(ctx, "DELETE", fmt.Sprintf("/v3/elb/loadbalancers/%s", id), nil)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			klog.V(4).InfoS("Load balancer already deleted", "id", id)
			return nil
		}

		if resp.StatusCode == http.StatusNoContent {
			klog.V(2).InfoS("Load balancer deleted", "id", id)
			return nil
		}

		// Retry on 409 Conflict — OTC returns this when listeners/pools are still being removed
		if resp.StatusCode == http.StatusConflict && attempt < maxRetries {
			wait := time.Duration(1<<uint(attempt)) * time.Second
			klog.V(2).InfoS("ELB delete conflict, retrying", "id", id, "attempt", attempt+1, "wait", wait, "body", string(body))
			select {
			case <-time.After(wait):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return fmt.Errorf("failed to delete load balancer: status %d, body: %s", resp.StatusCode, string(body))
	}

	return fmt.Errorf("failed to delete load balancer %s after %d retries", id, maxRetries)
}
// PoolDetails represents an ELB pool
type PoolDetails struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
}

// EnsurePool creates or finds a backend pool
func (c *ELBv3Client) EnsurePool(ctx context.Context, lbID, name, protocol string) (*PoolDetails, error) {
	klog.V(2).InfoS("Ensuring pool", "name", name, "lb", lbID)

	// Check if pool exists
	resp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/v3/elb/pools?name=%s&loadbalancer_id=%s", name, lbID), nil)
	if err == nil {
		defer resp.Body.Close()
		var result struct {
			Pools []PoolDetails `json:"pools"`
		}
		if json.NewDecoder(resp.Body).Decode(&result) == nil && len(result.Pools) > 0 {
			klog.V(2).InfoS("Pool already exists", "id", result.Pools[0].ID)
			return &result.Pools[0], nil
		}
	}

	// Create pool
	body := map[string]interface{}{
		"pool": map[string]interface{}{
			"name":            name,
			"protocol":        protocol,
			"lb_algorithm":    "ROUND_ROBIN",
			"loadbalancer_id": lbID,
		},
	}

	resp, err = c.makeRequest(ctx, "POST", "/v3/elb/pools", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create pool: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Pool PoolDetails `json:"pool"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode pool response: %w", err)
	}

	klog.V(2).InfoS("Pool created", "id", result.Pool.ID, "name", name)
	return &result.Pool, nil
}

// EnsureMembers adds node IPs as members to a pool
func (c *ELBv3Client) EnsureMembers(ctx context.Context, poolID string, nodes []*v1.Node, nodePort int32) error {
	klog.V(2).InfoS("Ensuring members", "pool", poolID, "nodes", len(nodes), "nodePort", nodePort)

	// List existing members
	resp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/v3/elb/pools/%s/members", poolID), nil)
	if err != nil {
		return fmt.Errorf("failed to list members: %w", err)
	}
	defer resp.Body.Close()

	var existing struct {
		Members []struct {
			ID      string `json:"id"`
			Address string `json:"address"`
		} `json:"members"`
	}
	json.NewDecoder(resp.Body).Decode(&existing)

	existingIPs := make(map[string]string) // IP -> member ID
	for _, m := range existing.Members {
		existingIPs[m.Address] = m.ID
	}

	// Get node internal IPs
	for _, node := range nodes {
		var nodeIP string
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				nodeIP = addr.Address
				break
			}
		}
		if nodeIP == "" {
			continue
		}

		// Skip if already exists
		if _, exists := existingIPs[nodeIP]; exists {
			delete(existingIPs, nodeIP) // Mark as still needed
			continue
		}

		// Add member
		body := map[string]interface{}{
			"member": map[string]interface{}{
				"address":       nodeIP,
				"protocol_port": nodePort,
				"subnet_cidr_id": c.config.Network.SubnetID,
			},
		}

		resp, err := c.makeRequest(ctx, "POST", fmt.Sprintf("/v3/elb/pools/%s/members", poolID), body)
		if err != nil {
			klog.V(2).InfoS("Failed to add member", "ip", nodeIP, "error", err)
			continue
		}
		resp.Body.Close()
		klog.V(2).InfoS("Member added", "ip", nodeIP, "port", nodePort)
	}

	// Delete stale members (nodes that were removed)
	for ip, memberID := range existingIPs {
		klog.V(2).InfoS("Removing stale member", "ip", ip, "id", memberID)
		resp, err := c.makeRequest(ctx, "DELETE", fmt.Sprintf("/v3/elb/pools/%s/members/%s", poolID, memberID), nil)
		if err != nil {
			klog.V(2).InfoS("Failed to delete stale member", "id", memberID, "error", err)
			continue
		}
		resp.Body.Close()
	}

	return nil
}

// ListListeners returns all listeners for a load balancer.
func (c *ELBv3Client) ListListeners(ctx context.Context, lbID string) ([]ListenerInfo, error) {
	resp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/v3/elb/listeners?loadbalancer_id=%s", lbID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Listeners []ListenerInfo `json:"listeners"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Listeners, nil
}

// ListenerInfo represents a listener with enough detail for sync operations.
type ListenerInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Protocol      string `json:"protocol"`
	ProtocolPort  int    `json:"protocol_port"`
	DefaultPoolID string `json:"default_pool_id"`
}

// DeleteListener deletes a single listener by ID.
func (c *ELBv3Client) DeleteListener(ctx context.Context, listenerID string) error {
	resp, err := c.makeRequest(ctx, "DELETE", fmt.Sprintf("/v3/elb/listeners/%s", listenerID), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// DeletePool deletes a pool and its members by ID.
func (c *ELBv3Client) DeletePool(ctx context.Context, poolID string) error {
	// Delete members first
	resp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/v3/elb/pools/%s/members", poolID), nil)
	if err == nil {
		defer resp.Body.Close()
		var result struct {
			Members []struct {
				ID string `json:"id"`
			} `json:"members"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		for _, m := range result.Members {
			r, e := c.makeRequest(ctx, "DELETE", fmt.Sprintf("/v3/elb/pools/%s/members/%s", poolID, m.ID), nil)
			if e == nil { r.Body.Close() }
		}
	}

	// Delete pool
	resp2, err := c.makeRequest(ctx, "DELETE", fmt.Sprintf("/v3/elb/pools/%s", poolID), nil)
	if err != nil {
		return err
	}
	resp2.Body.Close()
	return nil
}

// EnsureListener creates or finds a listener
func (c *ELBv3Client) EnsureListener(ctx context.Context, lbID, poolID, name, protocol string, port int) error {
	klog.V(2).InfoS("Ensuring listener", "name", name, "port", port)

	// Check if listener exists
	resp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/v3/elb/listeners?name=%s&loadbalancer_id=%s", name, lbID), nil)
	if err == nil {
		defer resp.Body.Close()
		var result struct {
			Listeners []struct {
				ID string `json:"id"`
			} `json:"listeners"`
		}
		if json.NewDecoder(resp.Body).Decode(&result) == nil && len(result.Listeners) > 0 {
			klog.V(2).InfoS("Listener already exists", "id", result.Listeners[0].ID)
			return nil
		}
	}

	// Create listener
	body := map[string]interface{}{
		"listener": map[string]interface{}{
			"name":            name,
			"protocol":        protocol,
			"protocol_port":   port,
			"loadbalancer_id": lbID,
			"default_pool_id": poolID,
		},
	}

	resp, err = c.makeRequest(ctx, "POST", "/v3/elb/listeners", body)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create listener: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	klog.V(2).InfoS("Listener created", "name", name, "port", port)
	return nil
}

// EnsureHealthMonitor creates a health monitor for a pool
// HealthMonitorConfig holds configurable parameters for health monitors.
// These can be set via Kubernetes service annotations.
type HealthMonitorConfig struct {
	Type       string // TCP, HTTP, HTTPS, UDP_CONNECT
	Delay      int    // Interval between checks (seconds)
	Timeout    int    // Timeout per check (seconds)
	MaxRetries int    // Failed checks before marking unhealthy
	URLPath    string // URL path for HTTP/HTTPS checks (default: "/")
	HTTPMethod string // HTTP method for HTTP/HTTPS checks (default: "GET")
}

// DefaultHealthMonitorConfig returns sensible defaults for a given protocol.
func DefaultHealthMonitorConfig(protocol string) *HealthMonitorConfig {
	cfg := &HealthMonitorConfig{
		Type:       "TCP",
		Delay:      5,
		Timeout:    10,
		MaxRetries: 3,
		URLPath:    "/",
		HTTPMethod: "GET",
	}

	switch protocol {
	case "HTTP":
		cfg.Type = "HTTP"
	case "HTTPS":
		cfg.Type = "HTTPS"
	case "UDP":
		cfg.Type = "UDP_CONNECT"
	}

	return cfg
}

func (c *ELBv3Client) EnsureHealthMonitor(ctx context.Context, poolID, protocol string) error {
	return c.EnsureHealthMonitorWithConfig(ctx, poolID, DefaultHealthMonitorConfig(protocol))
}

// EnsureHealthMonitorWithConfig creates a health monitor with full configuration.
// It first checks if one already exists for the pool to avoid duplicates.
func (c *ELBv3Client) EnsureHealthMonitorWithConfig(ctx context.Context, poolID string, cfg *HealthMonitorConfig) error {
	if cfg == nil {
		cfg = DefaultHealthMonitorConfig("TCP")
	}

	// Check for existing health monitor on this pool
	resp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/v3/elb/healthmonitors?pool_id=%s", poolID), nil)
	if err == nil {
		defer resp.Body.Close()
		var result struct {
			HealthMonitors []struct {
				ID string `json:"id"`
			} `json:"healthmonitors"`
		}
		if json.NewDecoder(resp.Body).Decode(&result) == nil && len(result.HealthMonitors) > 0 {
			klog.V(2).InfoS("Health monitor already exists", "pool", poolID, "id", result.HealthMonitors[0].ID)
			return nil
		}
	}

	// Build request body
	hm := map[string]interface{}{
		"pool_id":     poolID,
		"type":        cfg.Type,
		"delay":       cfg.Delay,
		"timeout":     cfg.Timeout,
		"max_retries": cfg.MaxRetries,
	}

	// Add HTTP-specific fields
	if cfg.Type == "HTTP" || cfg.Type == "HTTPS" {
		hm["url_path"] = cfg.URLPath
		hm["http_method"] = cfg.HTTPMethod
	}

	body := map[string]interface{}{"healthmonitor": hm}

	resp, err = c.makeRequest(ctx, "POST", "/v3/elb/healthmonitors", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health monitor create failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	klog.V(2).InfoS("Health monitor created", "pool", poolID, "type", cfg.Type,
		"delay", cfg.Delay, "timeout", cfg.Timeout, "maxRetries", cfg.MaxRetries)
	return nil
}


// DeleteListeners deletes all listeners for a load balancer
func (c *ELBv3Client) DeleteListeners(ctx context.Context, lbID string) error {
	klog.V(2).InfoS("Deleting listeners", "lb", lbID)

	resp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/v3/elb/listeners?loadbalancer_id=%s", lbID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Listeners []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"listeners"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	for _, l := range result.Listeners {
		klog.V(2).InfoS("Deleting listener", "id", l.ID, "name", l.Name)
		resp, err := c.makeRequest(ctx, "DELETE", fmt.Sprintf("/v3/elb/listeners/%s", l.ID), nil)
		if err != nil {
			return fmt.Errorf("failed to delete listener %s: %w", l.ID, err)
		}
		resp.Body.Close()
	}
	return nil
}

// DeletePools deletes all pools (and their members/monitors) for a load balancer
func (c *ELBv3Client) DeletePools(ctx context.Context, lbID string) error {
	klog.V(2).InfoS("Deleting pools", "lb", lbID)

	resp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/v3/elb/pools?loadbalancer_id=%s", lbID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Pools []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			HealthMonitor string `json:"healthmonitor_id"`
		} `json:"pools"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	for _, p := range result.Pools {
		// Delete health monitor first
		if p.HealthMonitor != "" {
			klog.V(2).InfoS("Deleting health monitor", "id", p.HealthMonitor)
			resp, err := c.makeRequest(ctx, "DELETE", fmt.Sprintf("/v3/elb/healthmonitors/%s", p.HealthMonitor), nil)
			if err == nil {
				resp.Body.Close()
			}
		}

		// Delete members
		mResp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/v3/elb/pools/%s/members", p.ID), nil)
		if err == nil {
			var members struct {
				Members []struct {
					ID string `json:"id"`
				} `json:"members"`
			}
			json.NewDecoder(mResp.Body).Decode(&members)
			mResp.Body.Close()
			for _, m := range members.Members {
				resp, err := c.makeRequest(ctx, "DELETE", fmt.Sprintf("/v3/elb/pools/%s/members/%s", p.ID, m.ID), nil)
				if err == nil {
					resp.Body.Close()
				}
			}
		}

		// Delete pool
		klog.V(2).InfoS("Deleting pool", "id", p.ID, "name", p.Name)
		resp, err := c.makeRequest(ctx, "DELETE", fmt.Sprintf("/v3/elb/pools/%s", p.ID), nil)
		if err != nil {
			return fmt.Errorf("failed to delete pool %s: %w", p.ID, err)
		}
		resp.Body.Close()
	}
	return nil
}

// EIPDetails represents an Elastic IP
type EIPDetails struct {
	ID              string `json:"id"`
	PublicIPAddress string `json:"public_ip_address"`
	Status          string `json:"status"`
	PortID          string `json:"port_id"`
}

// AssociateEIP binds an EIP to the ELB's VIP port
func (c *ELBv3Client) AssociateEIP(ctx context.Context, lbPortID string, bandwidth int) (*EIPDetails, error) {
	klog.V(2).InfoS("Creating and associating EIP", "portID", lbPortID, "bandwidth", bandwidth)

	if bandwidth <= 0 {
		bandwidth = 10 // Default 10 Mbit/s
	}

	// Create EIP and bind to port in one call
	body := map[string]interface{}{
		"publicip": map[string]interface{}{
			"type":    "5_bgp", // Swiss OTC BGP type
			"port_id": lbPortID,
		},
		"bandwidth": map[string]interface{}{
			"name":        "k8s-elb-bandwidth",
			"size":        bandwidth,
			"share_type":  "PER",
			"charge_mode": "traffic",
		},
	}

	// EIP API endpoint
	eipURL := fmt.Sprintf("https://vpc.%s.sc.otc.t-systems.com/v1/%s/publicips", c.config.Region, c.config.Auth.ProjectID)

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", eipURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.signOrTokenAuth(ctx, req); err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("EIP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("EIP creation failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		PublicIP EIPDetails `json:"publicip"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode EIP response: %w", err)
	}

	klog.V(2).InfoS("EIP created and associated", "eip", result.PublicIP.PublicIPAddress, "id", result.PublicIP.ID)
	return &result.PublicIP, nil
}

// ReleaseEIP releases an EIP by ID
func (c *ELBv3Client) ReleaseEIP(ctx context.Context, eipID string) error {
	klog.V(2).InfoS("Releasing EIP", "id", eipID)

	var resp *http.Response
	var err error

	if c.endpoints != nil && c.endpoints.VPC != "" {
		url := fmt.Sprintf("%s/v1/publicips/%s", c.endpoints.VPC, eipID)
		req, reqErr := http.NewRequestWithContext(ctx, "DELETE", url, nil)
		if reqErr != nil {
			return reqErr
		}
		if authErr := c.signOrTokenAuth(ctx, req); authErr != nil {
			return fmt.Errorf("auth failed: %w", authErr)
		}
		resp, err = c.httpClient.Do(req)
	} else {
		eipURL := fmt.Sprintf("https://vpc.%s.sc.otc.t-systems.com/v1/%s/publicips/%s", c.config.Region, c.config.Auth.ProjectID, eipID)
		req, reqErr := http.NewRequestWithContext(ctx, "DELETE", eipURL, nil)
		if reqErr != nil {
			return reqErr
		}
		if authErr := c.signOrTokenAuth(ctx, req); authErr != nil {
			return fmt.Errorf("auth failed: %w", authErr)
		}
		resp, err = c.httpClient.Do(req)
	}
	if err != nil {
		return fmt.Errorf("EIP release failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("EIP release failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	klog.V(2).InfoS("EIP released", "id", eipID)
	return nil
}

// FindEIPByPortID finds an EIP bound to a specific port
func (c *ELBv3Client) FindEIPByPortID(ctx context.Context, portID string) (*EIPDetails, error) {
	// Use VPC endpoint if available (testable), otherwise direct URL
	var resp *http.Response
	var err error

	if c.endpoints != nil && c.endpoints.VPC != "" {
		url := fmt.Sprintf("%s/v1/publicips?port_id=%s", c.endpoints.VPC, portID)
		req, reqErr := http.NewRequestWithContext(ctx, "GET", url, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		if authErr := c.signOrTokenAuth(ctx, req); authErr != nil {
			return nil, fmt.Errorf("auth failed: %w", authErr)
		}
		resp, err = c.httpClient.Do(req)
	} else {
		eipURL := fmt.Sprintf("https://vpc.%s.sc.otc.t-systems.com/v1/%s/publicips?port_id=%s", c.config.Region, c.config.Auth.ProjectID, portID)
		req, reqErr := http.NewRequestWithContext(ctx, "GET", eipURL, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		if authErr := c.signOrTokenAuth(ctx, req); authErr != nil {
			return nil, fmt.Errorf("auth failed: %w", authErr)
		}
		resp, err = c.httpClient.Do(req)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		PublicIPs []EIPDetails `json:"publicips"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	for _, eip := range result.PublicIPs {
		if eip.PortID == portID {
			return &eip, nil
		}
	}
	return nil, nil
}

// SecurityGroupRule represents a VPC security group rule
type SecurityGroupRule struct {
	ID              string `json:"id"`
	Direction       string `json:"direction"`
	Protocol        string `json:"protocol"`
	PortRangeMin    *int   `json:"port_range_min"`
	PortRangeMax    *int   `json:"port_range_max"`
	RemoteIPPrefix  string `json:"remote_ip_prefix"`
	SecurityGroupID string `json:"security_group_id"`
}

// PortDetails represents a Neutron port
type PortDetails struct {
	ID             string   `json:"id"`
	SecurityGroups []string `json:"security_groups"`
}

// GetPort retrieves port details including security groups
func (c *ELBv3Client) GetPort(ctx context.Context, portID string) (*PortDetails, error) {
	portURL := fmt.Sprintf("https://vpc.%s.sc.otc.t-systems.com/v2.0/ports/%s", c.config.Region, portID)

	req, err := http.NewRequestWithContext(ctx, "GET", portURL, nil)
	if err != nil {
		return nil, err
	}
	if err := c.signOrTokenAuth(ctx, req); err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Port PortDetails `json:"port"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result.Port, nil
}

// ListSecurityGroupRules lists rules for a security group
func (c *ELBv3Client) ListSecurityGroupRules(ctx context.Context, sgID string) ([]SecurityGroupRule, error) {
	sgURL := fmt.Sprintf("https://vpc.%s.sc.otc.t-systems.com/v1/%s/security-group-rules?security_group_id=%s",
		c.config.Region, c.config.Auth.ProjectID, sgID)

	req, err := http.NewRequestWithContext(ctx, "GET", sgURL, nil)
	if err != nil {
		return nil, err
	}
	if err := c.signOrTokenAuth(ctx, req); err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Rules []SecurityGroupRule `json:"security_group_rules"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Rules, nil
}

// EnsureSecurityGroupRule adds an ingress rule if not already present
func (c *ELBv3Client) EnsureSecurityGroupRule(ctx context.Context, sgID string, protocol string, port int, cidr string) error {
	klog.V(2).InfoS("Ensuring security group rule", "sg", sgID, "protocol", protocol, "port", port, "cidr", cidr)

	// Check if rule already exists
	rules, err := c.ListSecurityGroupRules(ctx, sgID)
	if err != nil {
		return fmt.Errorf("failed to list SG rules: %w", err)
	}

	for _, r := range rules {
		if r.Direction == "ingress" && r.Protocol == protocol &&
			r.PortRangeMin != nil && *r.PortRangeMin == port &&
			r.PortRangeMax != nil && *r.PortRangeMax == port &&
			r.RemoteIPPrefix == cidr {
			klog.V(4).InfoS("SG rule already exists", "port", port)
			return nil
		}
	}

	// Create new rule
	sgURL := fmt.Sprintf("https://vpc.%s.sc.otc.t-systems.com/v1/%s/security-group-rules",
		c.config.Region, c.config.Auth.ProjectID)

	body := map[string]interface{}{
		"security_group_rule": map[string]interface{}{
			"security_group_id": sgID,
			"direction":         "ingress",
			"protocol":          protocol,
			"port_range_min":    port,
			"port_range_max":    port,
			"remote_ip_prefix":  cidr,
		},
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", sgURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.signOrTokenAuth(ctx, req); err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SG rule creation failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SG rule creation failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	klog.V(2).InfoS("Security group rule created", "sg", sgID, "port", port, "cidr", cidr)
	return nil
}

// DeleteSecurityGroupRule deletes a security group rule by ID
func (c *ELBv3Client) DeleteSecurityGroupRule(ctx context.Context, ruleID string) error {
	sgURL := fmt.Sprintf("https://vpc.%s.sc.otc.t-systems.com/v1/%s/security-group-rules/%s",
		c.config.Region, c.config.Auth.ProjectID, ruleID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", sgURL, nil)
	if err != nil {
		return err
	}
	if err := c.signOrTokenAuth(ctx, req); err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
