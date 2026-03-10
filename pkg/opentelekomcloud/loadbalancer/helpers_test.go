package loadbalancer

import (
	"net/http"
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opentelekomcloud/cloud-provider-opentelekomcloud/pkg/opentelekomcloud/config"
)

// newTestConfig creates a Config that points all endpoints at the mock server.
func newTestConfig(mockURL string) *config.Config {
	return &config.Config{
		Auth: config.AuthConfig{
			AuthURL:        mockURL + "/v3",
			Username:       "test-user",
			Password:       "test-pass",
			ProjectName:    "test-project",
			ProjectID:      "project-0000",
			UserDomainName: "OTC00000000001000000000",
		},
		Region: "eu-ch2",
		Network: config.NetworkConfig{
			VpcID:     "vpc-test-0000",
			SubnetID:  "subnet-test-0000",
			NetworkID: "network-test-0000",
		},
		LoadBalancer: config.LoadBalancerConfig{
			AvailabilityZones: []string{"eu-ch2a", "eu-ch2b"},
		},
		Metadata: config.Metadata{
			ClusterID: "test-cluster",
		},
	}
}

// newTestClient creates an ELBv3Client backed by the mock server.
// The client's endpoints are overridden to use the mock URL.
func newTestClient(t *testing.T, mock *MockELBServer) *ELBv3Client {
	t.Helper()
	cfg := newTestConfig(mock.Server.URL)

	client := &ELBv3Client{
		config:     cfg,
		httpClient: &http.Client{},
		endpoints: &config.Endpoints{
			Identity: mock.Server.URL + "/v3",
			ELB:      mock.Server.URL,
			Compute:  mock.Server.URL,
			VPC:      mock.Server.URL,
		},
		authToken: mock.AuthToken,
	}

	return client
}

// newTestNode creates a Kubernetes Node with the given name and internal IP.
func newTestNode(name, internalIP string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1.NodeStatus{
			Addresses: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: internalIP,
				},
			},
		},
	}
}

// newTestService creates a Kubernetes Service of type LoadBalancer.
func newTestService(namespace, name string, ports []v1.ServicePort) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-uid-0000",
		},
		Spec: v1.ServiceSpec{
			Type:  v1.ServiceTypeLoadBalancer,
			Ports: ports,
		},
	}
}

// newTCPServicePort creates a TCP ServicePort helper.
func newTCPServicePort(name string, port, nodePort int32) v1.ServicePort {
	return v1.ServicePort{
		Name:     name,
		Protocol: v1.ProtocolTCP,
		Port:     port,
		NodePort: nodePort,
	}
}

// newTestNodes creates a slice of test nodes.
func newTestNodes(count int) []*v1.Node {
	nodes := make([]*v1.Node, count)
	for i := 0; i < count; i++ {
		nodes[i] = newTestNode(
			fmt.Sprintf("node-%d", i),
			fmt.Sprintf("10.0.1.%d", 10+i),
		)
	}
	return nodes
}

// assertRequestCount checks that the mock received the expected number of requests.
func assertRequestCount(t *testing.T, mock *MockELBServer, operation string, expected int) {
	t.Helper()
	mock.mu.Lock()
	defer mock.mu.Unlock()
	actual := mock.RequestCount[operation]
	if actual != expected {
		t.Errorf("expected %d %s requests, got %d", expected, operation, actual)
	}
}

// assertLoadBalancerCount checks the number of LBs in the mock.
func assertLoadBalancerCount(t *testing.T, mock *MockELBServer, expected int) {
	t.Helper()
	mock.mu.Lock()
	defer mock.mu.Unlock()
	actual := len(mock.LoadBalancers)
	if actual != expected {
		t.Errorf("expected %d load balancers, got %d", expected, actual)
	}
}

// assertPoolCount checks the number of pools in the mock.
func assertPoolCount(t *testing.T, mock *MockELBServer, expected int) {
	t.Helper()
	mock.mu.Lock()
	defer mock.mu.Unlock()
	actual := len(mock.Pools)
	if actual != expected {
		t.Errorf("expected %d pools, got %d", expected, actual)
	}
}

// assertMemberCount checks the number of members in a pool.
func assertMemberCount(t *testing.T, mock *MockELBServer, poolID string, expected int) {
	t.Helper()
	mock.mu.Lock()
	defer mock.mu.Unlock()
	actual := len(mock.Members[poolID])
	if actual != expected {
		t.Errorf("expected %d members in pool %s, got %d", expected, poolID, actual)
	}
}
