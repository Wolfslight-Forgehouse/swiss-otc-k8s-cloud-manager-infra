package config

import (
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v2"
)

// Config represents the configuration for Swiss OTC cloud provider
type Config struct {
	Auth         AuthConfig         `yaml:"auth"`
	Region       string             `yaml:"region"`
	Network      NetworkConfig      `yaml:"network"`
	LoadBalancer LoadBalancerConfig `yaml:"loadbalancer"`
	Metadata     Metadata           `yaml:"metadata"`
}

// NetworkConfig holds VPC and subnet configuration
type NetworkConfig struct {
	VpcID     string `yaml:"vpc_id"`
	SubnetID  string `yaml:"subnet_id"`
	NetworkID string `yaml:"network_id"`
}

// LoadBalancerConfig holds ELB-specific configuration
type LoadBalancerConfig struct {
	AvailabilityZones []string `yaml:"availability_zones"`
}

// AuthConfig contains authentication configuration for Swiss OTC
type AuthConfig struct {
	AuthURL          string `yaml:"auth_url"`
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
	AccessKey        string `yaml:"access_key"`
	SecretKey        string `yaml:"secret_key"`
	ProjectName      string `yaml:"project_name"`
	ProjectID        string `yaml:"project_id"`
	UserDomainName   string `yaml:"user_domain_name"`
	ProjectDomainID  string `yaml:"project_domain_id"`
}

// Metadata contains cluster metadata
type Metadata struct {
	ClusterID string `yaml:"cluster_id"`
}

// Endpoints contains Swiss OTC API endpoints
type Endpoints struct {
	Identity string
	ELB      string
	Compute  string
	VPC      string
}

// GetEndpoints returns the API endpoints for Swiss OTC based on region
func (c *Config) GetEndpoints() *Endpoints {
	region := c.Region
	if region == "" {
		region = "eu-ch2"
	}

	// Swiss OTC (eu-ch2)
	if region == "eu-ch2" {
		return &Endpoints{
			Identity: "https://iam-pub.eu-ch2.sc.otc.t-systems.com/v3",
			ELB:      "https://elb.eu-ch2.sc.otc.t-systems.com",
			Compute:  "https://ecs.eu-ch2.sc.otc.t-systems.com",
			VPC:      "https://vpc.eu-ch2.sc.otc.t-systems.com",
		}
	}

	// Flexible Engine fallback
	return &Endpoints{
		Identity: fmt.Sprintf("https://iam.%s.prod-cloud-ocb.orange-business.com/v3", region),
		ELB:      fmt.Sprintf("https://elb.%s.prod-cloud-ocb.orange-business.com", region),
		Compute:  fmt.Sprintf("https://ecs.%s.prod-cloud-ocb.orange-business.com", region),
		VPC:      fmt.Sprintf("https://vpc.%s.prod-cloud-ocb.orange-business.com", region),
	}
}

// LoadConfig reads and parses the cloud provider config
func LoadConfig(r io.Reader) (*Config, error) {
	if r == nil {
		return nil, errors.New("config reader is nil")
	}

	configData, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Set default region if not specified
	if config.Region == "" {
		config.Region = "eu-ch2"
	}

	// Set default auth URL for Swiss OTC if not specified
	if config.Auth.AuthURL == "" {
		config.Auth.AuthURL = "https://iam-pub.eu-ch2.sc.otc.t-systems.com/v3"
	}

	return &config, nil
}

// DefaultConfig returns a minimal valid Config with safe defaults.
// Useful for testing or when credentials are provided via environment variables.
func DefaultConfig() *Config {
	return &Config{
		Region: "eu-ch2",
		Auth: AuthConfig{
			AuthURL: "https://iam-pub.eu-ch2.sc.otc.t-systems.com/v3",
		},
	}
}


// validateConfig validates the configuration
func validateConfig(config *Config) error {
	hasPassword := config.Auth.Username != "" && config.Auth.Password != ""
	hasAKSK := config.Auth.AccessKey != "" && config.Auth.SecretKey != ""

	if !hasPassword && !hasAKSK {
		return errors.New("either username/password or access_key/secret_key is required")
	}
	if config.Auth.ProjectName == "" && config.Auth.ProjectID == "" {
		return errors.New("either project_name or project_id is required")
	}
	return nil
}

// Example configuration for Swiss OTC:
// auth:
//   auth_url: "https://iam-pub.eu-ch2.sc.otc.t-systems.com/v3"
//   username: "your-username"
//   password: "your-password"
//   project_name: "your-project"
//   user_domain_name: "OTC00000000001000000xxx"
// region: "eu-ch2"
// metadata:
//   cluster_id: "my-rke2-cluster"