package config

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/laindream/go-callflow-vis/log"
	"github.com/laindream/go-callflow-vis/mode"
)

type Config struct {
	PackagePrefix string   `toml:"package_prefix" json:"-"`
	Focus         mode.Set `toml:"focus" json:"focus"`
	Ignore        mode.Set `toml:"ignore" json:"ignore"`
	Layers        []*Layer `toml:"layer" json:"-"`
}

type Layer struct {
	Name     string    `toml:"name"`
	Entities []*Entity `toml:"entities"`
}

type Entity struct {
	Name      *mode.Set `toml:"name" json:"name,omitempty"`
	InSite    *mode.Set `toml:"in_site" json:"in_site,omitempty"`
	OutSite   *mode.Set `toml:"out_site" json:"out_site,omitempty"`
	Signature *mode.Set `toml:"signature" json:"signature,omitempty"`
}

func LoadConfig(path string) (*Config, error) {
	var config Config
	log.GetLogger().Debugf("loading config from %s", path)
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	log.GetLogger().Debugf("config loaded")
	return &config, nil
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if len(c.Layers) < 2 {
		return fmt.Errorf("number of layers must be at least 2")
	}
	for i, layer := range c.Layers {
		if len(layer.Entities) == 0 {
			return fmt.Errorf("layer %d has no entities", i)
		}
	}
	return nil
}
