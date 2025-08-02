package crawler

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
)

// ProxyType represents different types of proxies
type ProxyType string

const (
	HTTP   ProxyType = "http"
	HTTPS  ProxyType = "https"
	SOCKS4 ProxyType = "socks4"
	SOCKS5 ProxyType = "socks5"
)

// ProxyInfo contains detailed information about a proxy
type ProxyInfo struct {
	Address   string
	IP        string
	Port      string
	Type      ProxyType
	Country   string
	Anonymity string
	Latency   time.Duration
	LastCheck time.Time
	IsWorking bool
}

// ProxyManager manages proxy operations
type ProxyManager struct {
	proxies []ProxyInfo
}

// NewProxyManager creates a new proxy manager
func NewProxyManager() *ProxyManager {
	return &ProxyManager{
		proxies: make([]ProxyInfo, 0),
	}
}

// AddProxy adds a proxy to the manager
func (pm *ProxyManager) AddProxy(address string, proxyType ProxyType) {
	parts := strings.Split(address, ":")
	if len(parts) != 2 {
		return
	}

	proxy := ProxyInfo{
		Address:   address,
		IP:        parts[0],
		Port:      parts[1],
		Type:      proxyType,
		LastCheck: time.Now(),
	}

	pm.proxies = append(pm.proxies, proxy)
}

// AddProxies adds multiple proxies to the manager
func (pm *ProxyManager) AddProxies(addresses []string, proxyType ProxyType) {
	for _, address := range addresses {
		pm.AddProxy(address, proxyType)
	}
}

// GetProxies returns all proxies
func (pm *ProxyManager) GetProxies() []ProxyInfo {
	return pm.proxies
}

// GetWorkingProxies returns only working proxies
func (pm *ProxyManager) GetWorkingProxies() []ProxyInfo {
	var working []ProxyInfo
	for _, proxy := range pm.proxies {
		if proxy.IsWorking {
			working = append(working, proxy)
		}
	}
	return working
}

// GetProxiesByType returns proxies of a specific type
func (pm *ProxyManager) GetProxiesByType(proxyType ProxyType) []ProxyInfo {
	var filtered []ProxyInfo
	for _, proxy := range pm.proxies {
		if proxy.Type == proxyType {
			filtered = append(filtered, proxy)
		}
	}
	return filtered
}

// GetRandomProxy returns a random working proxy
func (pm *ProxyManager) GetRandomProxy() *ProxyInfo {
	working := pm.GetWorkingProxies()
	if len(working) == 0 {
		return nil
	}

	rand.Seed(time.Now().UnixNano())
	return &working[rand.Intn(len(working))]
}

// GetFastestProxies returns the fastest working proxies
func (pm *ProxyManager) GetFastestProxies(count int) []ProxyInfo {
	working := pm.GetWorkingProxies()

	// Sort by latency
	sort.Slice(working, func(i, j int) bool {
		return working[i].Latency < working[j].Latency
	})

	if len(working) <= count {
		return working
	}
	return working[:count]
}

// RemoveNonWorkingProxies removes proxies that are not working
func (pm *ProxyManager) RemoveNonWorkingProxies() {
	var working []ProxyInfo
	for _, proxy := range pm.proxies {
		if proxy.IsWorking {
			working = append(working, proxy)
		}
	}
	pm.proxies = working
}

// GetStats returns proxy statistics
func (pm *ProxyManager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	total := len(pm.proxies)
	working := len(pm.GetWorkingProxies())

	stats["total"] = total
	stats["working"] = working
	stats["success_rate"] = 0.0

	if total > 0 {
		stats["success_rate"] = float64(working) / float64(total) * 100
	}

	// Count by type
	typeCount := make(map[ProxyType]int)
	workingTypeCount := make(map[ProxyType]int)

	for _, proxy := range pm.proxies {
		typeCount[proxy.Type]++
		if proxy.IsWorking {
			workingTypeCount[proxy.Type]++
		}
	}

	stats["by_type"] = typeCount
	stats["working_by_type"] = workingTypeCount

	return stats
}

// PrintStats prints proxy statistics
func (pm *ProxyManager) PrintStats() {
	stats := pm.GetStats()

	fmt.Printf("\n📊 Proxy Statistics:\n")
	fmt.Printf("   Total proxies: %d\n", stats["total"])
	fmt.Printf("   Working proxies: %d\n", stats["working"])
	fmt.Printf("   Success rate: %.2f%%\n", stats["success_rate"])

	fmt.Printf("\n📋 By Type:\n")
	typeCount := stats["by_type"].(map[ProxyType]int)
	workingTypeCount := stats["working_by_type"].(map[ProxyType]int)

	for proxyType, count := range typeCount {
		working := workingTypeCount[proxyType]
		successRate := 0.0
		if count > 0 {
			successRate = float64(working) / float64(count) * 100
		}
		fmt.Printf("   %s: %d total, %d working (%.2f%%)\n",
			strings.ToUpper(string(proxyType)), count, working, successRate)
	}
}

// ExportAddresses exports proxy addresses as string slice
func (pm *ProxyManager) ExportAddresses() []string {
	var addresses []string
	for _, proxy := range pm.proxies {
		addresses = append(addresses, proxy.Address)
	}
	return addresses
}

// ExportWorkingAddresses exports working proxy addresses as string slice
func (pm *ProxyManager) ExportWorkingAddresses() []string {
	var addresses []string
	for _, proxy := range pm.GetWorkingProxies() {
		addresses = append(addresses, proxy.Address)
	}
	return addresses
}

// Clear removes all proxies from the manager
func (pm *ProxyManager) Clear() {
	pm.proxies = make([]ProxyInfo, 0)
}

// Count returns the total number of proxies
func (pm *ProxyManager) Count() int {
	return len(pm.proxies)
}

// WorkingCount returns the number of working proxies
func (pm *ProxyManager) WorkingCount() int {
	return len(pm.GetWorkingProxies())
}
