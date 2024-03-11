package config

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/laindream/go-callflow-vis/mode"
)

type Config struct {
	Focus  mode.ModeSet `toml:"focus" json:"focus"`
	Ignore mode.ModeSet `toml:"ignore" json:"ignore"`
	Layers []*Layer     `toml:"layer" json:"-"`
}

type Layer struct {
	Name     string    `toml:"name"`
	Entities []*Entity `toml:"entities"`
}

type Entity struct {
	Name      *mode.Mode `toml:"name"`
	InSite    *mode.Mode `toml:"in_site"`
	OutSite   *mode.Mode `toml:"out_site"`
	Signature *mode.Mode `toml:"signature"`
}

func LoadConfig(path string) (*Config, error) {
	var config Config
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
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
