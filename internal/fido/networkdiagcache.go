package fido

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var networkDiagramWWWRoot string

// SetNetworkDiagramCacheRoot sets the www root used to write cached map PNGs
// under static/network-maps/ (call from main after config load).
func SetNetworkDiagramCacheRoot(wwwRoot string) {
	networkDiagramWWWRoot = strings.TrimSpace(wwwRoot)
}

// DiagramCacheEntry is one cached PNG exposed to the web UI.
type DiagramCacheEntry struct {
	Key    string // full, hubs, hub-17
	NetNum int    // non-zero for per-net hub maps
}

// NetworkDiagramCacheDir returns .../static/network-maps.
func NetworkDiagramCacheDir(wwwRoot string) string {
	if strings.TrimSpace(wwwRoot) == "" {
		return ""
	}
	return filepath.Join(wwwRoot, "static", "network-maps")
}

// DiagramCacheWebURL returns a browser URL for a cached diagram PNG.
func DiagramCacheWebURL(network, key string) string {
	prefix := NetworkDiagPrefix(network)
	if prefix == "" || key == "" {
		return ""
	}
	return "/static/network-maps/" + prefix + "/" + key + ".png"
}

// WriteNetworkDiagramCache writes generated PNGs as static files for a network.
func WriteNetworkDiagramCache(wwwRoot, network string, zone int, pngs map[string][]byte) error {
	wwwRoot = strings.TrimSpace(wwwRoot)
	if wwwRoot == "" || len(pngs) == 0 {
		return nil
	}
	prefix := NetworkDiagPrefix(network)
	dir := filepath.Join(NetworkDiagramCacheDir(wwwRoot), prefix)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	entries, _ := filepath.Glob(filepath.Join(dir, "*.png"))
	for _, old := range entries {
		_ = os.Remove(old)
	}
	for filename, data := range pngs {
		key := diagramCacheWebKey(zone, prefix, filename)
		if key == "" {
			continue
		}
		path := filepath.Join(dir, key+".png")
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// WriteNetworkDiagramCacheDefault writes using the root from SetNetworkDiagramCacheRoot.
func WriteNetworkDiagramCacheDefault(network string, zone int, pngs map[string][]byte) error {
	return WriteNetworkDiagramCache(networkDiagramWWWRoot, network, zone, pngs)
}

// ListNetworkDiagramCache lists cached diagram keys for a network.
func ListNetworkDiagramCache(wwwRoot, network string, zone int) []DiagramCacheEntry {
	wwwRoot = strings.TrimSpace(wwwRoot)
	prefix := NetworkDiagPrefix(network)
	if wwwRoot == "" || prefix == "" {
		return nil
	}
	dir := filepath.Join(NetworkDiagramCacheDir(wwwRoot), prefix)
	matches, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil || len(matches) == 0 {
		return nil
	}
	var keys []string
	for _, path := range matches {
		base := strings.TrimSuffix(filepath.Base(path), ".png")
		if base != "" {
			keys = append(keys, base)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return diagramCacheSortKey(keys[i]) < diagramCacheSortKey(keys[j])
	})
	var out []DiagramCacheEntry
	for _, key := range keys {
		e := DiagramCacheEntry{Key: key}
		if strings.HasPrefix(key, "hub-") {
			if net, err := strconv.Atoi(strings.TrimPrefix(key, "hub-")); err == nil {
				e.NetNum = net
			}
		}
		_ = zone
		out = append(out, e)
	}
	return out
}

func diagramCacheSortKey(key string) string {
	switch key {
	case "full":
		return "0"
	case "hubs":
		return "1"
	}
	return "2:" + key
}

func diagramCacheWebKey(zone int, prefix, filename string) string {
	if filename == prefix+"_Full.png" {
		return "full"
	}
	if filename == prefix+"_Hubs.png" {
		return "hubs"
	}
	if strings.HasPrefix(filename, "Hub_") && strings.HasSuffix(filename, ".png") {
		base := strings.TrimSuffix(strings.TrimPrefix(filename, "Hub_"), ".png")
		parts := strings.SplitN(base, "-", 2)
		if len(parts) == 2 {
			if net, err := strconv.Atoi(parts[1]); err == nil {
				_ = zone
				return fmt.Sprintf("hub-%d", net)
			}
		}
	}
	return ""
}
