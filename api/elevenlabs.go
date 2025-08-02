package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ElevenLabsTester tests proxies against ElevenLabs API
type ElevenLabsTester struct {
	apiKey    string
	apiURL    string
	payload   string
	timeout   time.Duration
	userAgent string
}

// NewElevenLabsTester creates a new ElevenLabs API tester
func NewElevenLabsTester(apiKey, apiURL, payload string, timeout time.Duration) *ElevenLabsTester {
	return &ElevenLabsTester{
		apiKey:    apiKey,
		apiURL:    apiURL,
		payload:   payload,
		timeout:   timeout,
		userAgent: "RegProxy/1.0",
	}
}

// TestResult represents the result of testing a proxy with ElevenLabs API
type TestResult struct {
	Proxy       string
	IsWorking   bool
	StatusCode  int
	Latency     time.Duration
	Error       error
	ResponseLen int
}

// TestProxy tests a single proxy against ElevenLabs API
func (e *ElevenLabsTester) TestProxy(ctx context.Context, proxyAddr string) TestResult {
	result := TestResult{
		Proxy:     proxyAddr,
		IsWorking: false,
	}

	startTime := time.Now()

	// Create proxy URL
	proxyURL, err := url.Parse("http://" + proxyAddr)
	if err != nil {
		result.Error = fmt.Errorf("invalid proxy URL: %v", err)
		return result
	}

	// Create HTTP client with proxy
	transport := &http.Transport{
		Proxy:             http.ProxyURL(proxyURL),
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   e.timeout,
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", e.apiURL, strings.NewReader(e.payload))
	if err != nil {
		result.Error = fmt.Errorf("error creating request: %v", err)
		return result
	}

	// Set headers
	req.Header.Set("xi-api-key", e.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", e.userAgent)

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err
		return result
	}
	defer resp.Body.Close()

	result.Latency = time.Since(startTime)
	result.StatusCode = resp.StatusCode

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Errorf("error reading response: %v", err)
		return result
	}

	result.ResponseLen = len(body)

	// Check if request was successful
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.IsWorking = true
	} else {
		result.Error = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(200, len(body))]))
	}

	return result
}

// TestProxies tests multiple proxies concurrently
func (e *ElevenLabsTester) TestProxies(ctx context.Context, proxies []string, maxWorkers int) []TestResult {
	results := make([]TestResult, 0, len(proxies))
	resultsChan := make(chan TestResult, len(proxies))
	semaphore := make(chan struct{}, maxWorkers)

	// Start workers
	for _, proxy := range proxies {
		go func(proxyAddr string) {
			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-semaphore }()

			result := e.TestProxy(ctx, proxyAddr)
			resultsChan <- result
		}(proxy)
	}

	// Collect results
	for i := 0; i < len(proxies); i++ {
		select {
		case result := <-resultsChan:
			results = append(results, result)
		case <-ctx.Done():
			return results
		}
	}

	return results
}

// GetWorkingProxies returns only the working proxies from test results
func GetWorkingProxies(results []TestResult) []string {
	var working []string
	for _, result := range results {
		if result.IsWorking {
			working = append(working, result.Proxy)
		}
	}
	return working
}

// PrintResults prints test results in a formatted way
func PrintResults(results []TestResult, verbose bool) {
	working := 0
	failed := 0

	for _, result := range results {
		if result.IsWorking {
			working++
			if verbose {
				fmt.Printf("✅ %s - %dms - HTTP %d - %d bytes\n", 
					result.Proxy, result.Latency.Milliseconds(), result.StatusCode, result.ResponseLen)
			}
		} else {
			failed++
			if verbose {
				fmt.Printf("❌ %s - %v\n", result.Proxy, result.Error)
			}
		}
	}

	fmt.Printf("\n📊 ElevenLabs API Test Results:\n")
	fmt.Printf("   Working: %d\n", working)
	fmt.Printf("   Failed: %d\n", failed)
	fmt.Printf("   Success Rate: %.2f%%\n", float64(working)/float64(len(results))*100)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
