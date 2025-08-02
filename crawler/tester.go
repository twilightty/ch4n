package crawler

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ProxyTester handles proxy testing operations
type ProxyTester struct {
	testURL    string
	timeout    time.Duration
	maxWorkers int
}

// ProxyResult represents the result of a proxy test
type ProxyResult struct {
	Proxy     string
	IsWorking bool
	Latency   time.Duration
	Error     error
}

// NewProxyTester creates a new proxy tester
func NewProxyTester() *ProxyTester {
	return &ProxyTester{
		testURL:    "http://httpbin.org/ip",
		timeout:    10 * time.Second,
		maxWorkers: 50,
	}
}

// SetTestURL sets the URL to test against
func (pt *ProxyTester) SetTestURL(testURL string) {
	pt.testURL = testURL
}

// SetTimeout sets the test timeout
func (pt *ProxyTester) SetTimeout(timeout time.Duration) {
	pt.timeout = timeout
}

// SetMaxWorkers sets the maximum number of concurrent workers
func (pt *ProxyTester) SetMaxWorkers(workers int) {
	pt.maxWorkers = workers
}

// TestProxies tests a list of proxies and returns working ones
func (pt *ProxyTester) TestProxies(ctx context.Context, proxies []string) ([]string, error) {
	fmt.Printf("🔍 Testing %d proxies...\n", len(proxies))
	startTime := time.Now()

	results := make(chan ProxyResult, len(proxies))
	semaphore := make(chan struct{}, pt.maxWorkers)
	var wg sync.WaitGroup

	// Test proxies concurrently
	for _, proxy := range proxies {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-semaphore }()

			result := pt.testProxy(ctx, p)
			results <- result
		}(proxy)
	}

	// Close results channel when all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect working proxies
	var workingProxies []string
	totalTested := 0
	workingCount := 0

	for result := range results {
		totalTested++
		if result.IsWorking {
			workingProxies = append(workingProxies, result.Proxy)
			workingCount++
			fmt.Printf("✓ %s (%.2fms)\n", result.Proxy, float64(result.Latency.Nanoseconds())/1000000)
		} else if result.Error != nil {
			fmt.Printf("✗ %s: %v\n", result.Proxy, result.Error)
		}

		// Show progress every 10 tests
		if totalTested%10 == 0 {
			fmt.Printf("Progress: %d/%d tested, %d working\n", totalTested, len(proxies), workingCount)
		}
	}

	endTime := time.Now()
	fmt.Printf("\n📊 Test Results:\n")
	fmt.Printf("   Total tested: %d\n", totalTested)
	fmt.Printf("   Working proxies: %d\n", workingCount)
	fmt.Printf("   Success rate: %.2f%%\n", float64(workingCount)/float64(totalTested)*100)
	fmt.Printf("   Test time: %.2fs\n", endTime.Sub(startTime).Seconds())

	return workingProxies, nil
}

// testProxy tests a single proxy
func (pt *ProxyTester) testProxy(ctx context.Context, proxy string) ProxyResult {
	result := ProxyResult{
		Proxy:     proxy,
		IsWorking: false,
	}

	startTime := time.Now()

	// Create proxy URL
	proxyURL, err := url.Parse("http://" + proxy)
	if err != nil {
		result.Error = fmt.Errorf("invalid proxy URL: %v", err)
		return result
	}

	// Create HTTP client with proxy
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: (&net.Dialer{
			Timeout: pt.timeout,
		}).DialContext,
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   pt.timeout,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", pt.testURL, nil)
	if err != nil {
		result.Error = fmt.Errorf("error creating request: %v", err)
		return result
	}

	req.Header.Set("User-Agent", "ProxyTester/1.0")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err
		return result
	}
	defer resp.Body.Close()

	result.Latency = time.Since(startTime)

	if resp.StatusCode == http.StatusOK {
		result.IsWorking = true
	} else {
		result.Error = fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return result
}

// TestProxy tests a single proxy and returns the result
func (pt *ProxyTester) TestProxy(ctx context.Context, proxy string) ProxyResult {
	return pt.testProxy(ctx, proxy)
}

// FilterWorkingProxies filters a list of proxies to return only working ones
func (pt *ProxyTester) FilterWorkingProxies(ctx context.Context, proxies []string) ([]string, error) {
	return pt.TestProxies(ctx, proxies)
}
