package config

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/laindream/go-callflow-vis/ir"
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
	Name      *mode.Mode `toml:"name" json:"name,omitempty"`
	InSite    *mode.Mode `toml:"in_site" json:"in_site,omitempty"`
	OutSite   *mode.Mode `toml:"out_site" json:"out_site,omitempty"`
	Signature *mode.Mode `toml:"signature" json:"signature,omitempty"`
	Caller    *mode.Mode `toml:"caller" json:"caller,omitempty"`
	Callee    *mode.Mode `toml:"callee" json:"callee,omitempty"`
}

func (e *Entity) IsCheckIn() bool {
	if e.GetCaller() == nil && e.GetInSite() == nil {
		return false
	}
	return true
}

func (e *Entity) IsCheckOut() bool {
	if e.GetCallee() == nil && e.GetOutSite() == nil {
		return false
	}
	return true
}

func (e *Entity) GetName() *mode.Mode {
	if e == nil {
		return nil
	}
	return e.Name
}

func (e *Entity) GetInSite() *mode.Mode {
	if e == nil {
		return nil
	}
	return e.InSite
}

func (e *Entity) GetOutSite() *mode.Mode {
	if e == nil {
		return nil
	}
	return e.OutSite
}

func (e *Entity) GetSignature() *mode.Mode {
	if e == nil {
		return nil
	}
	return e.Signature
}

func (e *Entity) GetCaller() *mode.Mode {
	if e == nil {
		return nil
	}
	return e.Caller
}

func (e *Entity) GetCallee() *mode.Mode {
	if e == nil {
		return nil
	}
	return e.Callee
}

func (e *Entity) ShouldNodeSanityPass(n *ir.Node) bool {
	if n.GetFunc().GetName() == "" || n.GetFunc().GetSignature() == "" {
		return false
	}
	if e.GetName() == nil &&
		e.GetSignature() == nil &&
		!e.IsCheckIn() &&
		!e.IsCheckOut() {
		return false
	}
	nameCheckPass := true
	signatureCheckPass := true
	if e.GetName() != nil {
		nameCheckPass = false
		if e.GetName().Match(n.GetFunc().GetName()) {
			nameCheckPass = true
		}
	}
	if e.GetSignature() != nil {
		signatureCheckPass = false
		if e.GetSignature().Match(n.GetFunc().GetSignature()) {
			signatureCheckPass = true
		}
	}
	return nameCheckPass && signatureCheckPass
}

func (e *Entity) ShouldNodePass(n *ir.Node) bool {
	if n.GetFunc().GetName() == "" || n.GetFunc().GetSignature() == "" {
		return false
	}
	if e.GetName() == nil &&
		e.GetSignature() == nil &&
		!e.IsCheckIn() &&
		!e.IsCheckOut() {
		return false
	}
	nameCheckPass := true
	signatureCheckPass := true
	inCheckPass := true
	outCheckPass := true
	if e.GetName() != nil {
		nameCheckPass = false
		if e.GetName().Match(n.GetFunc().GetName()) {
			nameCheckPass = true
		}
	}
	if e.GetSignature() != nil {
		signatureCheckPass = false
		if e.GetSignature().Match(n.GetFunc().GetSignature()) {
			signatureCheckPass = true
		}
	}
	if e.IsCheckIn() {
		inCheckPass = false
		for _, eIn := range n.GetIn() {
			if e.ShouldInPass(eIn) {
				inCheckPass = true
				break
			}
		}
	}
	if e.IsCheckOut() {
		outCheckPass = false
		for _, eOut := range n.GetOut() {
			if e.ShouldOutPass(eOut) {
				outCheckPass = true
				break
			}
		}
	}
	return nameCheckPass && signatureCheckPass && inCheckPass && outCheckPass
}

func (e *Entity) ShouldInPass(in *ir.Edge) bool {
	if !e.IsCheckIn() {
		return true
	}
	isCallerCheckPass := true
	isInSiteCheckPass := true
	if e.GetCaller() != nil {
		isCallerCheckPass = false
		if e.GetCaller().Match(in.GetCaller().GetFunc().GetName()) {
			isCallerCheckPass = true
		}
	}
	if e.GetInSite() != nil {
		isInSiteCheckPass = false
		if e.GetInSite().Match(in.GetSite().GetName()) {
			isInSiteCheckPass = true
		}
	}
	return isCallerCheckPass && isInSiteCheckPass
}

func (e *Entity) ShouldOutPass(out *ir.Edge) bool {
	if !e.IsCheckOut() {
		return true
	}
	isCalleeCheckPass := true
	isOutSiteCheckPass := true
	if e.GetCallee() != nil {
		isCalleeCheckPass = false
		if e.GetCallee().Match(out.GetCallee().GetFunc().GetName()) {
			isCalleeCheckPass = true
		}
	}
	if e.GetOutSite() != nil {
		isOutSiteCheckPass = false
		if e.GetOutSite().Match(out.GetSite().GetName()) {
			isOutSiteCheckPass = true
		}
	}
	return isCalleeCheckPass && isOutSiteCheckPass
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
