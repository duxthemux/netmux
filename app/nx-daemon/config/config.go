package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"go.digitalcircle.com.br/dc/netmux/business/portforwarder"
)

// Config represents the agent central userconfig file.
type Config struct {
	Address   string    `json:"address"   yaml:"address,omitempty"`
	Network   string    `json:"network"   yaml:"network"`
	User      string    `json:"-"         yaml:"user,omitempty"`
	Cert      string    `json:"cert"      yaml:"cert,omitempty"`
	Key       string    `json:"key"       yaml:"key,omitempty"`
	IFace     string    `json:"iface"     yaml:"iface,omitempty"`
	LogLevel  string    `json:"logLevel"  yaml:"logLevel,omitempty"`
	LogFormat string    `json:"logFormat" yaml:"logFormat,omitempty"`
	Endpoints Endpoints `json:"endpoints" yaml:"endpoints,omitempty"`
}

type Endpoints []Endpoint

func (e Endpoints) FindByName(name string) (Endpoint, bool) {
	for _, v := range e {
		if v.Name == name {
			return v, true
		}
	}

	return Endpoint{}, false
}

type Endpoint struct {
	Name       string                       `yaml:"name"`
	Endpoint   string                       `yaml:"endpoint"`
	Kubernetes portforwarder.KubernetesInfo `yaml:"kubernetes"`
}

func New() *Config {
	return &Config{
		IFace:   DefaultIface,
		Address: "localhost:50000",
		Endpoints: []Endpoint{{
			Name:     "",
			Endpoint: "",
			Kubernetes: portforwarder.KubernetesInfo{
				Config:    "/home/USER/.kube/config",
				Namespace: "default",
				Endpoint:  "netmux",
				Context:   "netmux",
				Port:      "50000",
			},
		}},
	}
}

func (t *Config) ContextByName(n string) Endpoint {
	for _, v := range t.Endpoints {
		if v.Name == n {
			return v
		}
	}

	return Endpoint{}
}

func Load() (*Config, error) {
	cfg := New()
	fname := DefaultConfigPath

	if os.Getenv("CONFIG") != "" {
		fname = os.Getenv("CONFIG")
	}

	fileBytes, err := os.ReadFile(fname)
	if err != nil {
		return cfg, fmt.Errorf("error loading userconfig: %w", err)
	}

	err = yaml.Unmarshal(fileBytes, cfg)

	if err != nil {
		return cfg, fmt.Errorf("error unmashaling userconfig: %w", err)
	}

	if cfg.IFace == "" {
		cfg.IFace = DefaultIface
	}

	if cfg.Address == "" {
		cfg.Address = "localhost:50000"
	}

	if cfg.Network == "" {
		cfg.Network = "10.10.10.0/24"
	}

	return cfg, nil
}
