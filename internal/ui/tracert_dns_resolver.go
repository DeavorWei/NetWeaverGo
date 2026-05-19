//go:build windows

package ui

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
)

// DNSResolveResult represents the result of a DNS resolution for a single IP.
type DNSResolveResult struct {
	IP         string    `json:"ip"`
	Hostname   string    `json:"hostname,omitempty"`
	Error      string    `json:"error,omitempty"`
	ResolvedAt time.Time `json:"resolvedAt"`
}

// TracertDNSResolvedEvent represents the event data sent when DNS resolution completes.
type TracertDNSResolvedEvent struct {
	SessionID string             `json:"sessionId"`
	Results   []DNSResolveResult `json:"results"`
}

// TracertDNSResolver manages asynchronous DNS resolution for tracert sessions.
// It provides caching, deduplication, and event-based notification of results.
type TracertDNSResolver struct {
	mu            sync.RWMutex
	sessionID     string
	cache         map[string]*DNSResolveResult // IP -> Result
	pending       map[string]struct{}          // IPs currently being resolved
	maxConcurrent int                          // Maximum concurrent resolutions (0 = unlimited)
	eventBridge   func(eventType string, data interface{})
}

// NewTracertDNSResolver creates a new DNS resolver instance for a tracert session.
//
// Parameters:
//   - sessionID: Unique identifier for the tracert session
//   - eventBridge: Callback function to emit events to the frontend
//
// Returns:
//   - A new TracertDNSResolver instance ready for use
func NewTracertDNSResolver(sessionID string, eventBridge func(eventType string, data interface{})) *TracertDNSResolver {
	return &TracertDNSResolver{
		sessionID:     sessionID,
		cache:         make(map[string]*DNSResolveResult),
		pending:       make(map[string]struct{}),
		maxConcurrent: 0, // Default: unlimited
		eventBridge:   eventBridge,
	}
}

// ResolveAsync initiates asynchronous DNS resolution for the given IP addresses.
// This method returns immediately and performs DNS lookups in background goroutines.
// Results are delivered via the eventBridge callback with event type "tracert:dns-resolved".
//
// The method automatically:
//   - Filters out IPs that are already cached or pending resolution
//   - Marks new IPs as pending before starting resolution
//   - Updates cache and clears pending status on completion
//   - Sends results to frontend via eventBridge
//
// Parameters:
//   - ips: List of IP addresses to resolve
func (r *TracertDNSResolver) ResolveAsync(ips []string) {
	newIPs := r.collectNewIPs(ips)
	if len(newIPs) == 0 {
		return
	}

	// Mark IPs as pending
	r.mu.Lock()
	for _, ip := range newIPs {
		r.pending[ip] = struct{}{}
	}
	r.mu.Unlock()

	// Start async resolution for each new IP
	for _, ip := range newIPs {
		go r.resolveSingleIP(ip)
	}
}

// collectNewIPs filters out IPs that are already cached or currently being resolved.
// This ensures efficient resource usage by avoiding duplicate DNS queries.
//
// Parameters:
//   - ips: List of IP addresses to filter
//
// Returns:
//   - List of IP addresses that need DNS resolution
func (r *TracertDNSResolver) collectNewIPs(ips []string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var newIPs []string
	for _, ip := range ips {
		// Skip invalid IPs
		if ip == "" || ip == "*" {
			continue
		}

		// Check if already cached
		if _, cached := r.cache[ip]; cached {
			continue
		}

		// Check if already pending resolution
		if _, pending := r.pending[ip]; pending {
			continue
		}

		newIPs = append(newIPs, ip)
	}

	return newIPs
}

// GetCachedResult retrieves the cached DNS resolution result for a single IP.
//
// Parameters:
//   - ip: The IP address to look up
//
// Returns:
//   - The cached result, or nil if not found
func (r *TracertDNSResolver) GetCachedResult(ip string) *DNSResolveResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, exists := r.cache[ip]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	resultCopy := *result
	return &resultCopy
}

// GetAllCachedResults returns a copy of all cached DNS resolution results.
// This is useful for serialization or bulk updates.
//
// Returns:
//   - A map of IP addresses to their DNS resolution results
func (r *TracertDNSResolver) GetAllCachedResults() map[string]*DNSResolveResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a deep copy of the cache
	results := make(map[string]*DNSResolveResult, len(r.cache))
	for ip, result := range r.cache {
		resultCopy := *result
		results[ip] = &resultCopy
	}

	return results
}

// Clear removes all cached results and pending resolutions.
// This should be called when the tracert session ends or when a fresh start is needed.
func (r *TracertDNSResolver) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache = make(map[string]*DNSResolveResult)
	r.pending = make(map[string]struct{})
}

// SetMaxConcurrent sets the maximum number of concurrent DNS resolutions.
// A value of 0 means unlimited concurrency.
//
// Parameters:
//   - max: Maximum concurrent resolutions (0 = unlimited)
func (r *TracertDNSResolver) SetMaxConcurrent(max int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.maxConcurrent = max
}

// GetPendingCount returns the number of IPs currently being resolved.
// This is useful for monitoring and debugging.
//
// Returns:
//   - Number of pending DNS resolutions
func (r *TracertDNSResolver) GetPendingCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.pending)
}

// GetCachedCount returns the number of IPs in the cache.
// This is useful for monitoring and debugging.
//
// Returns:
//   - Number of cached DNS results
func (r *TracertDNSResolver) GetCachedCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.cache)
}

// IsCached checks if an IP has a cached DNS result.
//
// Parameters:
//   - ip: The IP address to check
//
// Returns:
//   - true if the IP has a cached result, false otherwise
func (r *TracertDNSResolver) IsCached(ip string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.cache[ip]
	return exists
}

// IsPending checks if an IP is currently being resolved.
//
// Parameters:
//   - ip: The IP address to check
//
// Returns:
//   - true if the IP is pending resolution, false otherwise
func (r *TracertDNSResolver) IsPending(ip string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.pending[ip]
	return exists
}

// resolveSingleIP performs the actual DNS reverse lookup for a single IP.
// This is called asynchronously by ResolveAsync.
//
// Parameters:
//   - ip: The IP address to resolve
func (r *TracertDNSResolver) resolveSingleIP(ip string) {
	result := r.performDNSLookup(ip)

	// Update cache and clear pending status
	r.mu.Lock()
	// Store result in cache (even if there was an error)
	r.cache[ip] = result
	// Remove from pending
	delete(r.pending, ip)
	r.mu.Unlock()

	// Emit event to frontend
	if r.eventBridge != nil {
		r.eventBridge("tracert:dns-resolved", TracertDNSResolvedEvent{
			SessionID: r.sessionID,
			Results:   []DNSResolveResult{*result},
		})
	}
}

// performDNSLookup executes the actual DNS reverse lookup using net.LookupAddr.
// It handles timeouts and errors gracefully.
//
// Parameters:
//   - ip: The IP address to look up
//
// Returns:
//   - A DNSResolveResult containing the lookup outcome
func (r *TracertDNSResolver) performDNSLookup(ip string) *DNSResolveResult {
	result := &DNSResolveResult{
		IP:         ip,
		ResolvedAt: time.Now(),
	}

	// Create context with timeout for DNS lookup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Perform reverse DNS lookup
	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil {
		// Log the error but don't fail the entire operation
		logger.Debug("TracertDNSResolver", ip, "DNS反向解析失败: %s", err.Error())
		result.Error = err.Error()
		return result
	}

	// If we got names, use the first one (trimmed of trailing dot)
	if len(names) > 0 {
		result.Hostname = strings.TrimSuffix(names[0], ".")
	}

	return result
}