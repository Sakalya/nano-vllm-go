package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Backends     []BackendConfig    `yaml:"backends"`
	LoadBalancer LoadBalancerConfig `yaml:"load_balancer"`
	HealthCheck  HealthCheckConfig  `yaml:"health_check"`
	Pool         PoolConfig         `yaml:"pool"`
}

type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type BackendConfig struct {
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight"`
}

type LoadBalancerConfig struct {
	Strategy string `yaml:"strategy"`
}

type HealthCheckConfig struct {
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
	Path     string        `yaml:"path"`
}

type PoolConfig struct {
	MaxIdleConnsPerHost int           `yaml:"max_idle_conns_per_host"`
	MaxConnsPerHost     int           `yaml:"max_conns_per_host"`
	IdleConnTimeout     time.Duration `yaml:"idle_conn_timeout"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.setDefaults()
	return &cfg, nil
}

func (c *Config) setDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 30 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 300 * time.Second
	}
	if c.LoadBalancer.Strategy == "" {
		c.LoadBalancer.Strategy = "round_robin"
	}
	if c.HealthCheck.Interval == 0 {
		c.HealthCheck.Interval = 10 * time.Second
	}
	if c.HealthCheck.Timeout == 0 {
		c.HealthCheck.Timeout = 5 * time.Second
	}
	if c.HealthCheck.Path == "" {
		c.HealthCheck.Path = "/health"
	}
	if c.Pool.MaxIdleConnsPerHost == 0 {
		c.Pool.MaxIdleConnsPerHost = 10
	}
	if c.Pool.MaxConnsPerHost == 0 {
		c.Pool.MaxConnsPerHost = 100
	}
	if c.Pool.IdleConnTimeout == 0 {
		c.Pool.IdleConnTimeout = 90 * time.Second
	}
	for i := range c.Backends {
		if c.Backends[i].Weight == 0 {
			c.Backends[i].Weight = 1
		}
	}
}
