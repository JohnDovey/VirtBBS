package config

import (
	"fmt"

	"github.com/virtbbs/virtbbs/internal/fido"
)

// RegisterFidoMappingHooks wires AreaFix/FileFix response handlers to persist
// area mappings in VirtBBS.DAT (internal/fido cannot import config).
func RegisterFidoMappingHooks() {
	fido.SetAreaMappingSaver(SaveAreaMapping)
	fido.SetFileAreaMappingSaver(SaveFileAreaMapping)
	fido.SetDownlinkPasswordSaver(SaveDownlinkPassword)
}

// SaveAreaMapping updates [fido.areas] or [[fido.networks]].areas.
func SaveAreaMapping(networkName, areaTag string, confID int, remove bool) error {
	cfg := Get()
	merged := *cfg
	if err := updateNetworkAreas(&merged, networkName, areaTag, confID, remove); err != nil {
		return err
	}
	return Save(&merged)
}

// SaveFileAreaMapping updates [fido.file_areas] or [[fido.networks]].file_areas.
func SaveFileAreaMapping(networkName, fileTag string, dirID int64, remove bool) error {
	cfg := Get()
	merged := *cfg
	if err := updateNetworkFileAreas(&merged, networkName, fileTag, int(dirID), remove); err != nil {
		return err
	}
	return Save(&merged)
}

func updateNetworkAreas(cfg *Config, networkName, areaTag string, confID int, remove bool) error {
	primary := cfg.Fido.EffectivePrimaryName()
	if networkName == primary || networkName == cfg.Fido.Name {
		if cfg.Fido.Areas == nil {
			cfg.Fido.Areas = map[string]int{}
		}
		if remove {
			delete(cfg.Fido.Areas, areaTag)
		} else {
			cfg.Fido.Areas[areaTag] = confID
		}
		return nil
	}
	for i := range cfg.Fido.Networks {
		if cfg.Fido.Networks[i].Name == networkName {
			if cfg.Fido.Networks[i].Areas == nil {
				cfg.Fido.Networks[i].Areas = map[string]int{}
			}
			if remove {
				delete(cfg.Fido.Networks[i].Areas, areaTag)
			} else {
				cfg.Fido.Networks[i].Areas[areaTag] = confID
			}
			return nil
		}
	}
	return fmt.Errorf("network %q not found in config", networkName)
}

// SaveDownlinkPassword updates the AreaFix password for a configured downlink.
func SaveDownlinkPassword(networkName, downlinkAddr, newPassword string) error {
	addr, err := fido.ParseAddr(downlinkAddr)
	if err != nil {
		return fmt.Errorf("invalid downlink address %q: %w", downlinkAddr, err)
	}
	cfg := Get()
	merged := *cfg
	if err := updateDownlinkPassword(&merged, networkName, addr, newPassword); err != nil {
		return err
	}
	return Save(&merged)
}

func updateDownlinkPassword(cfg *Config, networkName string, addr fido.Addr, newPassword string) error {
	updated := false
	primary := cfg.Fido.EffectivePrimaryName()
	if networkName == primary || networkName == cfg.Fido.Name {
		for i := range cfg.Fido.Downlinks {
			if cfg.Fido.Downlinks[i].MatchesAddr(addr) {
				cfg.Fido.Downlinks[i].Password = newPassword
				updated = true
			}
		}
	}
	for i := range cfg.Fido.Networks {
		if cfg.Fido.Networks[i].Name != networkName {
			continue
		}
		for j := range cfg.Fido.Networks[i].Downlinks {
			if cfg.Fido.Networks[i].Downlinks[j].MatchesAddr(addr) {
				cfg.Fido.Networks[i].Downlinks[j].Password = newPassword
				updated = true
			}
		}
	}
	if !updated {
		return fmt.Errorf("downlink %s not found in network %q", addr.String(), networkName)
	}
	return nil
}

func updateNetworkFileAreas(cfg *Config, networkName, fileTag string, dirID int, remove bool) error {
	primary := cfg.Fido.EffectivePrimaryName()
	if networkName == primary || networkName == cfg.Fido.Name {
		if cfg.Fido.FileAreas == nil {
			cfg.Fido.FileAreas = map[string]int{}
		}
		if remove {
			delete(cfg.Fido.FileAreas, fileTag)
		} else {
			cfg.Fido.FileAreas[fileTag] = dirID
		}
		return nil
	}
	for i := range cfg.Fido.Networks {
		if cfg.Fido.Networks[i].Name == networkName {
			if cfg.Fido.Networks[i].FileAreas == nil {
				cfg.Fido.Networks[i].FileAreas = map[string]int{}
			}
			if remove {
				delete(cfg.Fido.Networks[i].FileAreas, fileTag)
			} else {
				cfg.Fido.Networks[i].FileAreas[fileTag] = dirID
			}
			return nil
		}
	}
	return fmt.Errorf("network %q not found in config", networkName)
}
