package opentelekomcloud

import (
	"io"
	"fmt"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	"github.com/opentelekomcloud/cloud-provider-opentelekomcloud/pkg/opentelekomcloud/config"
	"github.com/opentelekomcloud/cloud-provider-opentelekomcloud/pkg/opentelekomcloud/loadbalancer"
	"github.com/opentelekomcloud/cloud-provider-opentelekomcloud/pkg/opentelekomcloud/instances"
)

const (
	ProviderName = "opentelekomcloud"
)

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		otc, err := NewOTC(config)
		if err != nil {
			return nil, err
		}
		return otc, nil
	})
}

// CloudProvider implements the cloudprovider.Interface for Swiss OTC
type CloudProvider struct {
	config       *config.Config
	loadBalancer cloudprovider.LoadBalancer
	instances    cloudprovider.Instances
	instancesV2  *instances.InstancesV2
}

// NewOTC creates a new Swiss OTC cloud provider instance
func NewOTC(cfg io.Reader) (*CloudProvider, error) {
	klog.V(2).InfoS("Initializing Swiss OTC Cloud Provider")

	config, err := config.LoadConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	lb, err := loadbalancer.NewLoadBalancer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create load balancer controller: %w", err)
	}

	inst, err := instances.NewInstances(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create instances controller: %w", err)
	}

	instV2, err := instances.NewInstancesV2(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create instances v2 controller: %w", err)
	}

	cp := &CloudProvider{
		config:       config,
		loadBalancer: lb,
		instances:    inst,
		instancesV2:  instV2,
	}

	klog.V(2).InfoS("Swiss OTC Cloud Provider initialized",
		"region", config.Region,
		"project", config.Auth.ProjectName)

	return cp, nil
}

func (cp *CloudProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	klog.V(2).InfoS("Swiss OTC Cloud Provider initialized with client builder")
}

func (cp *CloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return cp.loadBalancer, true
}

func (cp *CloudProvider) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (cp *CloudProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return cp.instancesV2, true
}

func (cp *CloudProvider) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

func (cp *CloudProvider) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (cp *CloudProvider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (cp *CloudProvider) ProviderName() string {
	return ProviderName
}

func (cp *CloudProvider) HasClusterID() bool {
	return cp.config.Metadata.ClusterID != ""
}
