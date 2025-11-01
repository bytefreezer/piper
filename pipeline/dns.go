package pipeline

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/n0needt0/go-goodies/log"
)

// DNSFilter performs DNS lookups to enrich events
type DNSFilter struct {
	ResolveFields []string
	Action        string // "replace" or "append"
	TargetField   string
	Nameservers   []string
	Timeout       time.Duration
	CacheSize     int
	CacheTTL      time.Duration
	cache         map[string]*dnsCacheEntry
	cacheMutex    sync.RWMutex
	resolver      *net.Resolver
}

type dnsCacheEntry struct {
	value     string
	timestamp time.Time
}

// NewDNSFilter creates a new DNS filter
func NewDNSFilter(config map[string]interface{}) (Filter, error) {
	filter := &DNSFilter{
		ResolveFields: make([]string, 0),
		Action:        "replace",
		Timeout:       2 * time.Second,
		CacheSize:     1000,
		CacheTTL:      3600 * time.Second,
		cache:         make(map[string]*dnsCacheEntry),
	}

	// Parse resolve fields
	if resolveFields, ok := config["resolve"].([]interface{}); ok {
		for _, field := range resolveFields {
			if fieldStr, ok := field.(string); ok {
				filter.ResolveFields = append(filter.ResolveFields, fieldStr)
			}
		}
	}

	// Parse action
	if action, ok := config["action"].(string); ok {
		filter.Action = action
	}

	// Parse target_field
	if targetField, ok := config["target_field"].(string); ok {
		filter.TargetField = targetField
	}

	// Parse nameservers
	if nameservers, ok := config["nameserver"].([]interface{}); ok {
		for _, ns := range nameservers {
			if nsStr, ok := ns.(string); ok {
				filter.Nameservers = append(filter.Nameservers, nsStr)
			}
		}
	}

	// Parse timeout
	if timeout, ok := config["timeout"].(float64); ok {
		filter.Timeout = time.Duration(timeout) * time.Second
	} else if timeoutInt, ok := config["timeout"].(int); ok {
		filter.Timeout = time.Duration(timeoutInt) * time.Second
	}

	// Parse cache_size
	if cacheSize, ok := config["cache_size"].(float64); ok {
		filter.CacheSize = int(cacheSize)
	} else if cacheSizeInt, ok := config["cache_size"].(int); ok {
		filter.CacheSize = cacheSizeInt
	}

	// Parse cache_ttl
	if cacheTTL, ok := config["cache_ttl"].(float64); ok {
		filter.CacheTTL = time.Duration(cacheTTL) * time.Second
	} else if cacheTTLInt, ok := config["cache_ttl"].(int); ok {
		filter.CacheTTL = time.Duration(cacheTTLInt) * time.Second
	}

	// Initialize resolver
	if len(filter.Nameservers) > 0 {
		// Use custom nameservers
		dialer := &net.Dialer{
			Timeout: filter.Timeout,
		}
		filter.resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				// Use first nameserver for simplicity
				return dialer.DialContext(ctx, "udp", filter.Nameservers[0]+":53")
			},
		}
	} else {
		// Use system resolver
		filter.resolver = net.DefaultResolver
	}

	return filter, nil
}

// Type returns the filter type
func (f *DNSFilter) Type() string {
	return "dns"
}

// Validate validates the filter configuration
func (f *DNSFilter) Validate(config map[string]interface{}) error {
	if _, ok := config["resolve"]; !ok {
		return fmt.Errorf("dns filter requires 'resolve' field list")
	}
	return nil
}

// Apply applies the DNS filter to a record
func (f *DNSFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()
	applied := false

	// Clean up expired cache entries
	f.cleanupCache()

	// Resolve each field
	for _, fieldName := range f.ResolveFields {
		fieldValue, exists := record[fieldName]
		if !exists {
			continue
		}

		// Convert to string
		fieldStr, ok := fieldValue.(string)
		if !ok {
			continue
		}

		// Check cache first
		if cached := f.getCached(fieldStr); cached != "" {
			f.applyResult(record, fieldName, cached)
			applied = true
			continue
		}

		// Perform DNS lookup with timeout
		lookupCtx, cancel := context.WithTimeout(context.Background(), f.Timeout)
		defer cancel()

		resolved, err := f.lookupHost(lookupCtx, fieldStr)
		if err != nil {
			log.Debugf("DNS filter: failed to resolve '%s': %v", fieldStr, err)
			continue
		}

		// Cache result
		f.setCached(fieldStr, resolved)

		// Apply result
		f.applyResult(record, fieldName, resolved)
		applied = true
	}

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  applied,
		Duration: time.Since(start),
	}, nil
}

// lookupHost performs forward or reverse DNS lookup
func (f *DNSFilter) lookupHost(ctx context.Context, host string) (string, error) {
	// Check if it's an IP address (reverse lookup)
	if ip := net.ParseIP(host); ip != nil {
		// Reverse lookup
		names, err := f.resolver.LookupAddr(ctx, host)
		if err != nil {
			return "", err
		}
		if len(names) > 0 {
			return names[0], nil
		}
		return "", fmt.Errorf("no PTR records found")
	}

	// Forward lookup
	addrs, err := f.resolver.LookupHost(ctx, host)
	if err != nil {
		return "", err
	}
	if len(addrs) > 0 {
		return addrs[0], nil
	}

	return "", fmt.Errorf("no A records found")
}

// applyResult applies DNS lookup result to record
func (f *DNSFilter) applyResult(record map[string]interface{}, fieldName, result string) {
	if f.TargetField != "" {
		// Store in target field
		record[f.TargetField] = result
	} else {
		// Replace or append to source field
		if f.Action == "append" {
			// Store both original and resolved
			record[fieldName+"_resolved"] = result
		} else {
			// Replace original field
			record[fieldName] = result
		}
	}
}

// getCached retrieves cached DNS result
func (f *DNSFilter) getCached(key string) string {
	f.cacheMutex.RLock()
	defer f.cacheMutex.RUnlock()

	if entry, exists := f.cache[key]; exists {
		if time.Since(entry.timestamp) < f.CacheTTL {
			return entry.value
		}
	}

	return ""
}

// setCached stores DNS result in cache
func (f *DNSFilter) setCached(key, value string) {
	f.cacheMutex.Lock()
	defer f.cacheMutex.Unlock()

	// Enforce cache size limit
	if len(f.cache) >= f.CacheSize {
		// Simple eviction: clear oldest entries
		f.evictOldest()
	}

	f.cache[key] = &dnsCacheEntry{
		value:     value,
		timestamp: time.Now(),
	}
}

// cleanupCache removes expired cache entries
func (f *DNSFilter) cleanupCache() {
	f.cacheMutex.Lock()
	defer f.cacheMutex.Unlock()

	now := time.Now()
	for key, entry := range f.cache {
		if now.Sub(entry.timestamp) > f.CacheTTL {
			delete(f.cache, key)
		}
	}
}

// evictOldest removes oldest cache entries
func (f *DNSFilter) evictOldest() {
	// Find oldest entry
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range f.cache {
		if oldestKey == "" || entry.timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.timestamp
		}
	}

	if oldestKey != "" {
		delete(f.cache, oldestKey)
	}
}
