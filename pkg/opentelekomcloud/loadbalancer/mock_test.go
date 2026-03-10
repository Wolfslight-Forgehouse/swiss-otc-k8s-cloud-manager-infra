package loadbalancer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// MockELBServer simulates the OTC ELB v3 API for offline testing.
// It tracks created resources and returns appropriate responses.
type MockELBServer struct {
	Server *httptest.Server
	mu     sync.Mutex

	// Resource stores
	LoadBalancers    map[string]*LoadBalancerDetails
	Pools            map[string]*PoolDetails
	Members          map[string][]mockMember // poolID -> members
	Listeners        map[string][]mockListener
	HealthMonitors   map[string]mockHealthMonitor
	EIPs             map[string]*EIPDetails
	Ports            map[string]*PortDetails
	SecurityGroups   map[string][]SecurityGroupRule
	AuthToken        string

	// Counters for assertions
	RequestCount map[string]int

	t *testing.T
}

type mockMember struct {
	ID            string `json:"id"`
	Address       string `json:"address"`
	ProtocolPort  int    `json:"protocol_port"`
	SubnetCIDRID  string `json:"subnet_cidr_id"`
}

type mockListener struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Protocol       string `json:"protocol"`
	ProtocolPort   int    `json:"protocol_port"`
	DefaultPoolID  string `json:"default_pool_id"`
	LoadbalancerID string `json:"-"` // internal tracking
}

type mockHealthMonitor struct {
	ID       string `json:"id"`
	PoolID   string `json:"pool_id"`
	Type     string `json:"type"`
	Delay    int    `json:"delay"`
	Timeout  int    `json:"timeout"`
	MaxRetries int  `json:"max_retries"`
}

// NewMockELBServer creates a new mock ELB API server.
func NewMockELBServer(t *testing.T) *MockELBServer {
	t.Helper()

	mock := &MockELBServer{
		LoadBalancers:  make(map[string]*LoadBalancerDetails),
		Pools:          make(map[string]*PoolDetails),
		Members:        make(map[string][]mockMember),
		Listeners:      make(map[string][]mockListener),
		HealthMonitors: make(map[string]mockHealthMonitor),
		EIPs:           make(map[string]*EIPDetails),
		Ports:          make(map[string]*PortDetails),
		SecurityGroups: make(map[string][]SecurityGroupRule),
		AuthToken:      "mock-token-12345",
		RequestCount:   make(map[string]int),
		t:              t,
	}

	mux := http.NewServeMux()

	// Identity (auth)
	mux.HandleFunc("/v3/auth/tokens", mock.handleAuth)

	// Load Balancers (project-scoped: /v3/{project_id}/elb/...)
	mux.HandleFunc("/v3/project-0000/elb/loadbalancers", mock.handleLoadBalancers)
	mux.HandleFunc("/v3/project-0000/elb/loadbalancers/", mock.handleLoadBalancerByID)

	// Pools
	mux.HandleFunc("/v3/project-0000/elb/pools", mock.handlePools)
	mux.HandleFunc("/v3/project-0000/elb/pools/", mock.handlePoolByID)

	// Listeners
	mux.HandleFunc("/v3/project-0000/elb/listeners", mock.handleListeners)
	mux.HandleFunc("/v3/project-0000/elb/listeners/", mock.handleListenerByID)

	// Health Monitors
	mux.HandleFunc("/v3/project-0000/elb/healthmonitors", mock.handleHealthMonitors)

	// EIPs (VPC)
	mux.HandleFunc("/v1/publicips", mock.handleEIPs)
	mux.HandleFunc("/v1/publicips/", mock.handleEIPByID)

	// Ports (VPC)
	mux.HandleFunc("/v2.0/ports/", mock.handlePorts)

	// Security Group Rules
	mux.HandleFunc("/v2.0/security-group-rules", mock.handleSecurityGroupRules)
	mux.HandleFunc("/v2.0/security-group-rules/", mock.handleSecurityGroupRuleByID)

	mock.Server = httptest.NewServer(mux)
	t.Cleanup(func() { mock.Server.Close() })

	return mock
}

// track increments the request counter for a given operation.
// Must be called while m.mu is already held.
func (m *MockELBServer) track(op string) {
	m.RequestCount[op]++
}

// nextID generates a deterministic mock UUID.
// Must be called while m.mu is already held.
func (m *MockELBServer) nextID(prefix string) string {
	count := m.RequestCount["_id_"+prefix]
	m.RequestCount["_id_"+prefix] = count + 1
	return fmt.Sprintf("%s-%04d-0000-0000-000000000000", prefix, count)
}

// ── Auth ────────────────────────────────────────────────────
func (m *MockELBServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.track("auth")
	w.Header().Set("X-Subject-Token", m.AuthToken)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": map[string]interface{}{
			"expires_at": "2099-12-31T23:59:59.000000Z",
		},
	})
}

// ── Load Balancers ──────────────────────────────────────────
func (m *MockELBServer) handleLoadBalancers(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		m.track("GetLoadBalancers")
		name := r.URL.Query().Get("name")
		var lbs []*LoadBalancerDetails
		for _, lb := range m.LoadBalancers {
			if name == "" || lb.Name == name {
				lbs = append(lbs, lb)
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"loadbalancers": lbs})

	case http.MethodPost:
		m.track("CreateLoadBalancer")
		var body struct {
			LB CreateLoadBalancerRequest `json:"loadbalancer"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id := m.nextID("lb")
		lb := &LoadBalancerDetails{
			ID:                 id,
			Name:               body.LB.Name,
			Description:        body.LB.Description,
			VIPAddress:         "192.168.1.100",
			VIPPortID:          fmt.Sprintf("port-%s", id),
			VIPSubnetCIDRID:    body.LB.VIPSubnetCIDRID,
			Provider:           "vlb",
			OperatingStatus:    "ONLINE",
			ProvisioningStatus: "ACTIVE",
		}
		m.LoadBalancers[id] = lb
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"loadbalancer": lb})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *MockELBServer) handleLoadBalancerByID(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := strings.TrimPrefix(r.URL.Path, "/v3/project-0000/elb/loadbalancers/")
	id = strings.Split(id, "/")[0]

	switch r.Method {
	case http.MethodGet:
		m.track("GetLoadBalancer")
		lb, ok := m.LoadBalancers[id]
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"loadbalancer": lb})

	case http.MethodDelete:
		m.track("DeleteLoadBalancer")
		delete(m.LoadBalancers, id)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ── Pools ───────────────────────────────────────────────────
func (m *MockELBServer) handlePools(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		m.track("GetPools")
		name := r.URL.Query().Get("name")
		lbID := r.URL.Query().Get("loadbalancer_id")
		var pools []*PoolDetails
		for _, p := range m.Pools {
			if (name == "" || p.Name == name) && (lbID == "" || true) {
				pools = append(pools, p)
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"pools": pools})

	case http.MethodPost:
		m.track("CreatePool")
		var body struct {
			Pool struct {
				Name           string `json:"name"`
				Protocol       string `json:"protocol"`
				LBAlgorithm    string `json:"lb_algorithm"`
				LoadbalancerID string `json:"loadbalancer_id"`
			} `json:"pool"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		id := m.nextID("pool")
		pool := &PoolDetails{
			ID:       id,
			Name:     body.Pool.Name,
			Protocol: body.Pool.Protocol,
		}
		m.Pools[id] = pool
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"pool": pool})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *MockELBServer) handlePoolByID(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := strings.TrimPrefix(r.URL.Path, "/v3/project-0000/elb/pools/")
	parts := strings.Split(path, "/")
	poolID := parts[0]

	// /v3/elb/pools/{pool_id}/members
	if len(parts) >= 2 && parts[1] == "members" {
		m.handleMembers(w, r, poolID, parts)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		m.track("DeletePool")
		delete(m.Pools, poolID)
		delete(m.Members, poolID)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMembers is called from handlePoolByID which already holds m.mu.
func (m *MockELBServer) handleMembers(w http.ResponseWriter, r *http.Request, poolID string, parts []string) {
	switch r.Method {
	case http.MethodGet:
		m.track("GetMembers")
		members := m.Members[poolID]
		if members == nil {
			members = []mockMember{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"members": members})

	case http.MethodPost:
		m.track("CreateMember")
		var body struct {
			Member struct {
				Address      string `json:"address"`
				ProtocolPort int    `json:"protocol_port"`
				SubnetCIDRID string `json:"subnet_cidr_id"`
			} `json:"member"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		member := mockMember{
			ID:           m.nextID("member"),
			Address:      body.Member.Address,
			ProtocolPort: body.Member.ProtocolPort,
			SubnetCIDRID: body.Member.SubnetCIDRID,
		}
		m.Members[poolID] = append(m.Members[poolID], member)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"member": member})

	case http.MethodDelete:
		// /v3/elb/pools/{pool_id}/members/{member_id}
		if len(parts) >= 3 {
			m.track("DeleteMember")
			memberID := parts[2]
			members := m.Members[poolID]
			for i, mem := range members {
				if mem.ID == memberID {
					m.Members[poolID] = append(members[:i], members[i+1:]...)
					break
				}
			}
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ── Listeners ───────────────────────────────────────────────
func (m *MockELBServer) handleListeners(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		m.track("GetListeners")
		lbID := r.URL.Query().Get("loadbalancer_id")
		var listeners []mockListener
		for _, ls := range m.Listeners {
			for _, l := range ls {
				if lbID == "" || l.LoadbalancerID == lbID {
					listeners = append(listeners, l)
				}
			}
		}
		if listeners == nil {
			listeners = []mockListener{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"listeners": listeners})

	case http.MethodPost:
		m.track("CreateListener")
		var body struct {
			Listener struct {
				Name              string `json:"name"`
				Protocol          string `json:"protocol"`
				ProtocolPort      int    `json:"protocol_port"`
				LoadbalancerID    string `json:"loadbalancer_id"`
				DefaultPoolID     string `json:"default_pool_id"`
			} `json:"listener"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		id := m.nextID("listener")
		listener := mockListener{
			ID:             id,
			Name:           body.Listener.Name,
			Protocol:       body.Listener.Protocol,
			ProtocolPort:   body.Listener.ProtocolPort,
			DefaultPoolID:  body.Listener.DefaultPoolID,
			LoadbalancerID: body.Listener.LoadbalancerID,
		}
		m.Listeners[body.Listener.LoadbalancerID] = append(
			m.Listeners[body.Listener.LoadbalancerID], listener,
		)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"listener": listener})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *MockELBServer) handleListenerByID(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := strings.TrimPrefix(r.URL.Path, "/v3/project-0000/elb/listeners/")

	if r.Method == http.MethodDelete {
		m.track("DeleteListener")
		for lbID, ls := range m.Listeners {
			for i, l := range ls {
				if l.ID == id {
					m.Listeners[lbID] = append(ls[:i], ls[i+1:]...)
					break
				}
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// ── Health Monitors ─────────────────────────────────────────
func (m *MockELBServer) handleHealthMonitors(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		m.track("GetHealthMonitors")
		poolID := r.URL.Query().Get("pool_id")
		var monitors []mockHealthMonitor
		for _, hm := range m.HealthMonitors {
			if poolID == "" || hm.PoolID == poolID {
				monitors = append(monitors, hm)
			}
		}
		if monitors == nil {
			monitors = []mockHealthMonitor{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"healthmonitors": monitors})

	case http.MethodPost:
		m.track("CreateHealthMonitor")
		var body struct {
			HM struct {
				PoolID     string `json:"pool_id"`
				Type       string `json:"type"`
				Delay      int    `json:"delay"`
				Timeout    int    `json:"timeout"`
				MaxRetries int    `json:"max_retries"`
			} `json:"healthmonitor"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		id := m.nextID("hm")
		hm := mockHealthMonitor{
			ID:         id,
			PoolID:     body.HM.PoolID,
			Type:       body.HM.Type,
			Delay:      body.HM.Delay,
			Timeout:    body.HM.Timeout,
			MaxRetries: body.HM.MaxRetries,
		}
		m.HealthMonitors[id] = hm
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"healthmonitor": hm})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ── EIPs ────────────────────────────────────────────────────
func (m *MockELBServer) handleEIPs(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		m.track("GetEIPs")
		portID := r.URL.Query().Get("port_id")
		var eips []*EIPDetails
		for _, e := range m.EIPs {
			if portID == "" || e.PortID == portID {
				eips = append(eips, e)
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"publicips": eips})

	case http.MethodPost:
		m.track("CreateEIP")
		id := m.nextID("eip")
		eip := &EIPDetails{
			ID:             id,
			PublicIPAddress: "80.74.100.42",
			Status:         "ACTIVE",
		}
		m.EIPs[id] = eip
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"publicip": eip})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *MockELBServer) handleEIPByID(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := strings.TrimPrefix(r.URL.Path, "/v1/publicips/")

	switch r.Method {
	case http.MethodGet:
		m.track("GetEIP")
		eip, ok := m.EIPs[id]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"publicip": eip})
	case http.MethodDelete:
		m.track("DeleteEIP")
		delete(m.EIPs, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ── Ports ───────────────────────────────────────────────────
func (m *MockELBServer) handlePorts(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := strings.TrimPrefix(r.URL.Path, "/v2.0/ports/")
	m.track("GetPort")

	port, ok := m.Ports[id]
	if !ok {
		port = &PortDetails{
			ID:             id,
			SecurityGroups: []string{"sg-default-0000"},
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"port": port})
}

// ── Security Group Rules ────────────────────────────────────
func (m *MockELBServer) handleSecurityGroupRules(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		m.track("GetSecurityGroupRules")
		sgID := r.URL.Query().Get("security_group_id")
		rules := m.SecurityGroups[sgID]
		if rules == nil {
			rules = []SecurityGroupRule{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"security_group_rules": rules})

	case http.MethodPost:
		m.track("CreateSecurityGroupRule")
		var body struct {
			Rule struct {
				Direction       string `json:"direction"`
				Protocol        string `json:"protocol"`
				PortRangeMin    *int   `json:"port_range_min"`
				PortRangeMax    *int   `json:"port_range_max"`
				RemoteIPPrefix  string `json:"remote_ip_prefix"`
				SecurityGroupID string `json:"security_group_id"`
			} `json:"security_group_rule"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		id := m.nextID("sgr")
		rule := SecurityGroupRule{
			ID:              id,
			Direction:       body.Rule.Direction,
			Protocol:        body.Rule.Protocol,
			PortRangeMin:    body.Rule.PortRangeMin,
			PortRangeMax:    body.Rule.PortRangeMax,
			RemoteIPPrefix:  body.Rule.RemoteIPPrefix,
			SecurityGroupID: body.Rule.SecurityGroupID,
		}
		m.SecurityGroups[body.Rule.SecurityGroupID] = append(
			m.SecurityGroups[body.Rule.SecurityGroupID], rule,
		)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"security_group_rule": rule})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *MockELBServer) handleSecurityGroupRuleByID(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := strings.TrimPrefix(r.URL.Path, "/v2.0/security-group-rules/")

	if r.Method == http.MethodDelete {
		m.track("DeleteSecurityGroupRule")
		for sgID, rules := range m.SecurityGroups {
			for i, rule := range rules {
				if rule.ID == id {
					m.SecurityGroups[sgID] = append(rules[:i], rules[i+1:]...)
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
