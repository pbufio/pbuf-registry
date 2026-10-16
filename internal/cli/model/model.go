package model

import (
	"os"

	"gopkg.in/yaml.v3"
)

const (
	PbufConfigFilename = "pbuf.yaml"
)

type Config struct {
	Version  string    `yaml:"version,omitempty"`
	Name     string    `yaml:"name,omitempty"`
	Registry Registry  `yaml:"registry,omitempty"`
	Export   Export    `yaml:"export,omitempty"`
	Modules  []*Module `yaml:"modules,omitempty"`
}

type Export struct {
	Paths []string `yaml:"paths,omitempty"`
}

type Registry struct {
	Addr     string       `yaml:"addr,omitempty"`
	Insecure bool         `yaml:"insecure,omitempty"`
	OIDC     RegistryOIDC `yaml:"oidc,omitempty"`
}

type RegistryOIDC struct {
	Issuer   string `yaml:"issuer,omitempty"`
	ClientID string `yaml:"client_id,omitempty"`
}

type Module struct {
	Name                 string `yaml:"name,omitempty"`
	Repository           string `yaml:"repository,omitempty"`
	Path                 string `yaml:"path,omitempty"`
	Branch               string `yaml:"branch,omitempty"`
	Tag                  string `yaml:"tag,omitempty"`
	OutputFolder         string `yaml:"out,omitempty"`
	GenerateOutputFolder string `yaml:"gen_out,omitempty"`
}

func (c *Config) HasRegistry() bool {
	return c.Registry.Addr != ""
}

func (c *Config) Save() error {
	// encode to yaml and save to file PbufConfigFilename
	pbufYamlFile, err := os.OpenFile(PbufConfigFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = pbufYamlFile.Close() }()

	encoder := yaml.NewEncoder(pbufYamlFile)
	encoder.SetIndent(2)
	if err := encoder.Encode(c); err != nil {
		_ = encoder.Close()
		return err
	}

	return encoder.Close()
}
