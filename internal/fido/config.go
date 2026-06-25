// ============================================================================
// VirtBBS — A modern BBS server inspired by PCBoard BBS
//           (Clark Development Company, 1987-1996)
//
// Copyright (c) 2026 John Dovey <dovey.john@gmail.com>
//
// MIT License
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.
//
// Change History:
//   v0.0.3  2026-06-24  Phase 9: FidoNet area configuration
//   v0.0.5  2026-06-24  Add json tags so API returns clean lowercase keys
//   v0.0.6  2026-06-24  Add NodelistPath, BinkpPort, Networks for multi-network support
// ============================================================================

package fido

// Config holds all FidoNet settings for VirtBBS.
// The top-level fields describe the primary (first) network.
// Additional networks are listed in Networks[].
type Config struct {
	Enabled     bool           `toml:"enabled"      json:"enabled"`
	Address     string         `toml:"address"      json:"address"`
	Uplink      string         `toml:"uplink"       json:"uplink"`
	Password    string         `toml:"password"     json:"password"`
	InboundDir  string         `toml:"inbound_dir"  json:"inbound_dir"`
	OutboundDir string         `toml:"outbound_dir" json:"outbound_dir"`
	NodelistDir string         `toml:"nodelist_dir" json:"nodelist_dir"` // dir holding NODELIST.xxx
	BinkpPort   int            `toml:"binkp_port"   json:"binkp_port"`   // default 24554
	Areas       map[string]int `toml:"areas"        json:"areas"`

	// Networks lists additional FidoNet-compatible networks (LovlyNet, etc.).
	// Each entry is a fully independent network with its own address space.
	Networks []NetworkDef `toml:"networks" json:"networks"`
}

// NetworkDef describes one additional FidoNet-compatible network.
type NetworkDef struct {
	Name        string         `toml:"name"         json:"name"`
	Enabled     bool           `toml:"enabled"      json:"enabled"`
	Address     string         `toml:"address"      json:"address"`
	Uplink      string         `toml:"uplink"       json:"uplink"`
	Password    string         `toml:"password"     json:"password"`
	InboundDir  string         `toml:"inbound_dir"  json:"inbound_dir"`
	OutboundDir string         `toml:"outbound_dir" json:"outbound_dir"`
	NodelistDir string         `toml:"nodelist_dir" json:"nodelist_dir"`
	BinkpPort   int            `toml:"binkp_port"   json:"binkp_port"`
	Areas       map[string]int `toml:"areas"        json:"areas"`
}

// DefaultConfig returns a Config with sensible disabled defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:     false,
		Address:     "1:1/1",
		InboundDir:  "fido/inbound",
		OutboundDir: "fido/outbound",
		NodelistDir: "fido/nodelist",
		BinkpPort:   24554,
		Areas:       map[string]int{},
	}
}

// NodeAddr parses this node's configured address.
func (c *Config) NodeAddr() Addr {
	if !c.Enabled || c.Address == "" {
		return Addr{}
	}
	a, _ := ParseAddr(c.Address)
	return a
}

// UplinkAddr parses the uplink address.
func (c *Config) UplinkAddr() Addr {
	if c.Uplink == "" {
		return Addr{}
	}
	a, _ := ParseAddr(c.Uplink)
	return a
}

// ConferenceForArea returns the conference ID mapped to an area tag, -1 if not found.
func (c *Config) ConferenceForArea(tag string) int {
	id, ok := c.Areas[tag]
	if !ok {
		return -1
	}
	return id
}

// AllNetworks returns the primary network plus all additional networks as
// a flat slice of NetworkDef. Used when iterating all configured networks.
func (c *Config) AllNetworks() []NetworkDef {
	primary := NetworkDef{
		Name:        "FidoNet",
		Enabled:     c.Enabled,
		Address:     c.Address,
		Uplink:      c.Uplink,
		Password:    c.Password,
		InboundDir:  c.InboundDir,
		OutboundDir: c.OutboundDir,
		NodelistDir: c.NodelistDir,
		BinkpPort:   c.BinkpPort,
		Areas:       c.Areas,
	}
	result := []NetworkDef{primary}
	result = append(result, c.Networks...)
	return result
}

// NetworkByName finds a NetworkDef by name (case-insensitive).
// Returns the primary network for an empty name.
func (c *Config) NetworkByName(name string) *NetworkDef {
	all := c.AllNetworks()
	for i := range all {
		if name == "" || strEqFold(all[i].Name, name) {
			return &all[i]
		}
	}
	return nil
}

func strEqFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// NetworkDef helpers

func (n *NetworkDef) NodeAddr() Addr {
	if n.Address == "" {
		return Addr{}
	}
	a, _ := ParseAddr(n.Address)
	return a
}

func (n *NetworkDef) UplinkAddr() Addr {
	if n.Uplink == "" {
		return Addr{}
	}
	a, _ := ParseAddr(n.Uplink)
	return a
}

func (n *NetworkDef) ConferenceForArea(tag string) int {
	id, ok := n.Areas[tag]
	if !ok {
		return -1
	}
	return id
}

func (n *NetworkDef) Port() int {
	if n.BinkpPort <= 0 {
		return 24554
	}
	return n.BinkpPort
}
