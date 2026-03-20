package loadbalancer

import (
	"context"
	"testing"
	"fmt"

	v1 "k8s.io/api/core/v1"
)

func TestMockServer_CreateAndGetLoadBalancer(t *testing.T) {
	mock := NewMockELBServer(t)
	client := newTestClient(t, mock)
	ctx := context.Background()

	// Create
	req := &CreateLoadBalancerRequest{
		Name:                 "test-lb",
		VIPSubnetCIDRID:      "subnet-test-0000",
		AvailabilityZoneList: []string{"eu-ch2a"},
	}
	lb, err := client.CreateLoadBalancer(ctx, req, 10)
	if err != nil {
		t.Fatalf("CreateLoadBalancer failed: %v", err)
	}
	if lb.Name != "test-lb" {
		t.Errorf("expected name 'test-lb', got '%s'", lb.Name)
	}
	if lb.ID == "" {
		t.Error("expected non-empty ID")
	}
	if lb.VIPAddress == "" {
		t.Error("expected non-empty VIP address")
	}

	// Get by name
	found, err := client.GetLoadBalancer(ctx, "test-lb")
	if err != nil {
		t.Fatalf("GetLoadBalancer failed: %v", err)
	}
	if found.ID != lb.ID {
		t.Errorf("expected ID %s, got %s", lb.ID, found.ID)
	}

	// Not found
	_, err = client.GetLoadBalancer(ctx, "non-existent")
	if err == nil {
		t.Error("expected error for non-existent LB")
	}

	assertLoadBalancerCount(t, mock, 1)
	assertRequestCount(t, mock, "CreateLoadBalancer", 1)
}

func TestMockServer_DeleteLoadBalancer(t *testing.T) {
	mock := NewMockELBServer(t)
	client := newTestClient(t, mock)
	ctx := context.Background()

	// Create then delete
	req := &CreateLoadBalancerRequest{
		Name:                 "to-delete",
		VIPSubnetCIDRID:      "subnet-0000",
		AvailabilityZoneList: []string{"eu-ch2a"},
	}
	lb, err := client.CreateLoadBalancer(ctx, req, 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	assertLoadBalancerCount(t, mock, 1)

	err = client.DeleteLoadBalancer(ctx, lb.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	assertLoadBalancerCount(t, mock, 0)
}

func TestMockServer_PoolLifecycle(t *testing.T) {
	mock := NewMockELBServer(t)
	client := newTestClient(t, mock)
	ctx := context.Background()

	pool, err := client.EnsurePool(ctx, "lb-0000", "test-pool", "TCP")
	if err != nil {
		t.Fatalf("EnsurePool: %v", err)
	}
	if pool.Name != "test-pool" {
		t.Errorf("expected pool name 'test-pool', got '%s'", pool.Name)
	}
	if pool.Protocol != "TCP" {
		t.Errorf("expected protocol TCP, got %s", pool.Protocol)
	}
	assertPoolCount(t, mock, 1)
}

func TestMockServer_MemberManagement(t *testing.T) {
	mock := NewMockELBServer(t)
	client := newTestClient(t, mock)
	ctx := context.Background()

	// Create pool first
	pool, err := client.EnsurePool(ctx, "lb-0000", "members-test", "TCP")
	if err != nil {
		t.Fatalf("EnsurePool: %v", err)
	}

	// Add members via EnsureMembers
	nodes := newTestNodes(3)
	err = client.EnsureMembers(ctx, pool.ID, nodes, 30080)
	if err != nil {
		t.Fatalf("EnsureMembers: %v", err)
	}
	assertMemberCount(t, mock, pool.ID, 3)
	assertRequestCount(t, mock, "CreateMember", 3)
}

func TestMockServer_ListenerLifecycle(t *testing.T) {
	mock := NewMockELBServer(t)
	client := newTestClient(t, mock)
	ctx := context.Background()

	err := client.EnsureListener(ctx, "lb-0000", "pool-0000", "test-listener", "TCP", 80)
	if err != nil {
		t.Fatalf("EnsureListener: %v", err)
	}
	assertRequestCount(t, mock, "CreateListener", 1)

	// Delete
	err = client.DeleteListeners(ctx, "lb-0000")
	if err != nil {
		t.Fatalf("DeleteListeners: %v", err)
	}
}

func TestMockServer_HealthMonitor(t *testing.T) {
	mock := NewMockELBServer(t)
	client := newTestClient(t, mock)
	ctx := context.Background()

	err := client.EnsureHealthMonitor(ctx, "pool-0000", "TCP")
	if err != nil {
		t.Fatalf("EnsureHealthMonitor: %v", err)
	}
	assertRequestCount(t, mock, "CreateHealthMonitor", 1)
}

func TestHelpers_NewTestNode(t *testing.T) {
	node := newTestNode("worker-1", "10.0.1.50")

	if node.Name != "worker-1" {
		t.Errorf("expected name 'worker-1', got '%s'", node.Name)
	}

	var ip string
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			ip = addr.Address
		}
	}
	if ip != "10.0.1.50" {
		t.Errorf("expected IP '10.0.1.50', got '%s'", ip)
	}
}

func TestHelpers_NewTestService(t *testing.T) {
	svc := newTestService("default", "my-service", []v1.ServicePort{
		newTCPServicePort("http", 80, 30080),
		newTCPServicePort("https", 443, 30443),
	})

	if svc.Name != "my-service" {
		t.Errorf("expected name 'my-service', got '%s'", svc.Name)
	}
	if svc.Spec.Type != v1.ServiceTypeLoadBalancer {
		t.Errorf("expected type LoadBalancer, got %s", svc.Spec.Type)
	}
	if len(svc.Spec.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(svc.Spec.Ports))
	}
	if svc.Spec.Ports[0].NodePort != 30080 {
		t.Errorf("expected nodePort 30080, got %d", svc.Spec.Ports[0].NodePort)
	}
}

func TestHelpers_NewTestNodes(t *testing.T) {
	nodes := newTestNodes(5)
	if len(nodes) != 5 {
		t.Fatalf("expected 5 nodes, got %d", len(nodes))
	}
	// Check IPs are sequential
	for i, node := range nodes {
		var ip string
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				ip = addr.Address
			}
		}
		expected := fmt.Sprintf("10.0.1.%d", 10+i)
		if ip != expected {
			t.Errorf("node %d: expected IP %s, got %s", i, expected, ip)
		}
	}
}

func TestHelpers_NewTestConfig(t *testing.T) {
	cfg := newTestConfig("http://localhost:9999")

	if cfg.Auth.Username != "test-user" {
		t.Errorf("unexpected username: %s", cfg.Auth.Username)
	}
	if cfg.Region != "eu-ch2" {
		t.Errorf("unexpected region: %s", cfg.Region)
	}
	if cfg.Network.VpcID != "vpc-test-0000" {
		t.Errorf("unexpected VPC ID: %s", cfg.Network.VpcID)
	}
	if len(cfg.LoadBalancer.AvailabilityZones) != 2 {
		t.Errorf("expected 2 AZs, got %d", len(cfg.LoadBalancer.AvailabilityZones))
	}
}
