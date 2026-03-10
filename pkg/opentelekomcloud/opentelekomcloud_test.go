package opentelekomcloud

import (
	"strings"
	"testing"

	cloudprovider "k8s.io/cloud-provider"
)

const testConfig = `
auth:
  auth_url: "https://iam-pub.eu-ch2.sc.otc.t-systems.com/v3"
  username: "test-user"
  password: "test-password"
  project_name: "test-project"
  user_domain_name: "OTC00000000001000000000"
region: "eu-ch2"
network:
  vpc_id: "test-vpc-id"
  subnet_id: "test-subnet-id"
metadata:
  cluster_id: "test-cluster"
`

func TestProviderRegistered(t *testing.T) {
	provider, err := cloudprovider.GetCloudProvider(ProviderName, strings.NewReader(testConfig))
	if err != nil {
		t.Fatalf("provider %q not registered: %v", ProviderName, err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
}

func TestNewOTC(t *testing.T) {
	cp, err := NewOTC(strings.NewReader(testConfig))
	if err != nil {
		t.Fatalf("NewOTC returned error: %v", err)
	}
	if cp == nil {
		t.Fatal("NewOTC returned nil")
	}
}

func TestNewOTCNilConfig(t *testing.T) {
	_, err := NewOTC(nil)
	if err == nil {
		t.Fatal("expected error with nil config, got nil")
	}
}

func TestProviderName(t *testing.T) {
	if ProviderName != "opentelekomcloud" {
		t.Fatalf("expected provider name %q, got %q", "opentelekomcloud", ProviderName)
	}
}
