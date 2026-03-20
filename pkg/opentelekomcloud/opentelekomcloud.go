/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package opentelekomcloud

import (
	"fmt"
	"io"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	"github.com/opentelekomcloud/cloud-provider-opentelekomcloud/pkg/opentelekomcloud/config"
	"github.com/opentelekomcloud/cloud-provider-opentelekomcloud/pkg/opentelekomcloud/loadbalancer"
)

const (
	ProviderName = "opentelekomcloud"
)

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(cfg io.Reader) (cloudprovider.Interface, error) {
		otc, err := NewOTC(cfg)
		if err != nil {
			return nil, err
		}
		return otc, nil
	})
}

// CloudProvider implements the cloudprovider.Interface for Open Telekom Cloud.
type CloudProvider struct {
	cfg *config.Config
	lb  *loadbalancer.LoadBalancer
}

// NewOTC creates a new CloudProvider from the given config reader.
// If cfgReader is nil, a default (empty) config is used; credentials must be supplied later.
func NewOTC(cfgReader io.Reader) (*CloudProvider, error) {
	var cfg *config.Config
	if cfgReader != nil {
		var err error
		cfg, err = config.LoadConfig(cfgReader)
		if err != nil {
			return nil, fmt.Errorf("failed to read cloud config: %w", err)
		}
	} else {
		cfg = config.DefaultConfig()
	}

	lb, err := loadbalancer.NewLoadBalancer(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LoadBalancer controller: %w", err)
	}

	klog.V(2).InfoS("Open Telekom Cloud provider initialized",
		"region", cfg.Region,
	)

	return &CloudProvider{cfg: cfg, lb: lb}, nil
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping or run custom controllers specific to the cloud provider.
func (otc *CloudProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
}

// ProviderName returns the cloud provider ID.
func (otc *CloudProvider) ProviderName() string {
	return ProviderName
}

// LoadBalancer returns a balancer interface.
func (otc *CloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return otc.lb, true
}

// Instances returns an instances interface — not yet implemented.
func (otc *CloudProvider) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

// InstancesV2 returns a InstancesV2 interface — not yet implemented.
func (otc *CloudProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}

// Zones returns a zones interface — not yet implemented.
func (otc *CloudProvider) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

// Clusters returns a clusters interface — not implemented.
func (otc *CloudProvider) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface — not implemented.
func (otc *CloudProvider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// HasClusterID returns true if a ClusterID is required and set.
func (otc *CloudProvider) HasClusterID() bool {
	return false
}
