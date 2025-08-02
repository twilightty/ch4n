package crawler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProxySource represents a proxy source with URL and pattern
type ProxySource struct {
	URL     string
	Pattern string
}

// ProxyResponse represents the response from a JSON API
type ProxyResponse struct {
	Data    []ProxyItem `json:"data"`
	Proxies []ProxyItem `json:"proxies"`
}

// ProxyItem represents a single proxy from JSON API
type ProxyItem struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
}

// Crawler handles proxy crawling operations
type Crawler struct {
	sources    []ProxySource
	httpClient *http.Client
	userAgent  string
	maxWorkers int
	timeout    time.Duration
}

// NewCrawler creates a new proxy crawler
func NewCrawler() *Crawler {
	return &Crawler{
		sources:    getProxySources(),
		maxWorkers: 10,
		timeout:    15 * time.Second,
		userAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// SetMaxWorkers sets the maximum number of concurrent workers
func (c *Crawler) SetMaxWorkers(workers int) {
	c.maxWorkers = workers
}

// SetTimeout sets the HTTP request timeout
func (c *Crawler) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
	c.httpClient.Timeout = timeout
}

// CrawlProxies crawls proxies from all sources
func (c *Crawler) CrawlProxies(ctx context.Context) ([]string, error) {
	fmt.Println("🚀 Starting proxy crawling from sources...")
	startTime := time.Now()

	allProxies := make(map[string]bool)
	var mu sync.Mutex

	// Create a channel to limit concurrent workers
	semaphore := make(chan struct{}, c.maxWorkers)
	var wg sync.WaitGroup

	for _, source := range c.sources {
		wg.Add(1)
		go func(src ProxySource) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-semaphore }()

			proxies := c.fetchProxiesFromSource(ctx, src)

			mu.Lock()
			for _, proxy := range proxies {
				allProxies[proxy] = true
			}
			mu.Unlock()

			fmt.Printf("✓ %s: %d proxies\n", src.URL, len(proxies))
		}(source)
	}

	wg.Wait()

	// Convert map to slice and validate
	var validProxies []string
	for proxy := range allProxies {
		if c.validateProxy(proxy) {
			validProxies = append(validProxies, proxy)
		}
	}

	sort.Strings(validProxies)

	endTime := time.Now()
	fmt.Printf("\n📊 Results:\n")
	fmt.Printf("   Total proxies found: %d\n", len(allProxies))
	fmt.Printf("   Valid proxies: %d\n", len(validProxies))
	fmt.Printf("   Execution time: %.2fs\n", endTime.Sub(startTime).Seconds())

	return validProxies, nil
}

// fetchProxiesFromSource fetches proxies from a single source
func (c *Crawler) fetchProxiesFromSource(ctx context.Context, source ProxySource) []string {
	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
	if err != nil {
		fmt.Printf("✗ %s: error creating request: %v\n", source.URL, err)
		return nil
	}

	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Printf("✗ %s: %v\n", source.URL, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("✗ %s: HTTP %d\n", source.URL, resp.StatusCode)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("✗ %s: error reading response: %v\n", source.URL, err)
		return nil
	}

	return c.parseProxies(string(body), source.Pattern)
}

// parseProxies parses proxies from response body
func (c *Crawler) parseProxies(body, pattern string) []string {
	var proxies []string

	if pattern == "json" {
		proxies = c.parseJSONProxies(body)
	} else {
		proxies = c.parseTextProxies(body, pattern)
	}

	return proxies
}

// parseJSONProxies parses proxies from JSON response
func (c *Crawler) parseJSONProxies(body string) []string {
	var proxies []string
	var response ProxyResponse

	if err := json.Unmarshal([]byte(body), &response); err != nil {
		// Try parsing as array
		var items []ProxyItem
		if err := json.Unmarshal([]byte(body), &items); err != nil {
			return proxies
		}
		response.Data = items
	}

	// Handle different JSON structures
	items := response.Data
	if len(items) == 0 {
		items = response.Proxies
	}

	for _, item := range items {
		if item.IP != "" && item.Port != "" {
			proxies = append(proxies, fmt.Sprintf("%s:%s", item.IP, item.Port))
		}
	}

	return proxies
}

// parseTextProxies parses proxies from text response using regex
func (c *Crawler) parseTextProxies(body, pattern string) []string {
	var proxies []string

	re, err := regexp.Compile(pattern)
	if err != nil {
		return proxies
	}

	matches := re.FindAllStringSubmatch(body, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			proxy := fmt.Sprintf("%s:%s", match[1], match[2])
			proxies = append(proxies, proxy)
		}
	}

	return proxies
}

// validateProxy validates if a proxy string is in correct format
func (c *Crawler) validateProxy(proxy string) bool {
	parts := strings.Split(proxy, ":")
	if len(parts) != 2 {
		return false
	}

	ip := parts[0]
	port := parts[1]

	// Validate IP address
	if net.ParseIP(ip) == nil {
		return false
	}

	// Validate port
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return false
	}

	return true
}

// SaveToFile saves proxies to a file
func (c *Crawler) SaveToFile(proxies []string, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for _, proxy := range proxies {
		if _, err := writer.WriteString(proxy + "\n"); err != nil {
			return fmt.Errorf("error writing to file: %v", err)
		}
	}

	return nil
}

// LoadFromFile loads proxies from a file
func (c *Crawler) LoadFromFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	var proxies []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && c.validateProxy(line) {
			proxies = append(proxies, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return proxies, nil
}

// GetSampleProxies returns a sample of proxies for display
func (c *Crawler) GetSampleProxies(proxies []string, count int) []string {
	if len(proxies) <= count {
		return proxies
	}
	return proxies[:count]
}

// getProxySources returns all proxy sources
func getProxySources() []ProxySource {
	return []ProxySource{
		// HTTP/HTTPS proxies - Updated and new sources
		{"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://github.com/zloi-user/hideip.me/raw/refs/heads/master/http.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://github.com/zloi-user/hideip.me/raw/refs/heads/master/https.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://github.com/zloi-user/hideip.me/raw/refs/heads/master/connect.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/ErcinDedeoglu/proxies/main/proxies/http.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/ErcinDedeoglu/proxies/main/proxies/https.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/vakhov/fresh-proxy-list/master/http.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/vakhov/fresh-proxy-list/master/https.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/main/http.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/jetkai/proxy-list/main/online-proxies/txt/proxies-http.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/jetkai/proxy-list/main/online-proxies/txt/proxies-https.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/ShiftyTR/Proxy-List/master/http.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/ShiftyTR/Proxy-List/master/https.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},

		// New HTTP/HTTPS sources
		{"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/http.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/https.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/roosterkid/openproxylist/main/HTTPS_RAW.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/roosterkid/openproxylist/main/HTTP_RAW.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/hookzof/socks5_list/master/proxy.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/clarketm/proxy-list/master/proxy-list-raw.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/sunny9577/proxy-scraper/master/proxies.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/zevtyardt/proxy-list/main/http.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/zevtyardt/proxy-list/main/https.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/almroot/proxylist/master/list.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/rdavydov/proxy-list/main/proxies_anonymous/http.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/rdavydov/proxy-list/main/proxies_anonymous/https.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},

		// SOCKS4 proxies
		{"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks4.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://github.com/zloi-user/hideip.me/raw/refs/heads/master/socks4.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/ErcinDedeoglu/proxies/main/proxies/socks4.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/vakhov/fresh-proxy-list/master/socks4.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/socks4.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},

		// SOCKS5 proxies
		{"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks5.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://github.com/zloi-user/hideip.me/raw/refs/heads/master/socks5.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/ErcinDedeoglu/proxies/main/proxies/socks5.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/vakhov/fresh-proxy-list/master/socks5.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
		{"https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/socks5.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},

		// API-based sources (JSON format)
		{"https://proxylist.geonode.com/api/proxy-list?limit=500&page=1&sort_by=lastChecked&sort_type=desc&filterUpTime=90&protocols=http%2Chttps%2Csocks4%2Csocks5", "json"},
		{"https://api.proxyscrape.com/v2/?request=get&protocol=http&timeout=10000&country=all&ssl=all&anonymity=all", "json"},
		{"https://api.proxyscrape.com/v2/?request=get&protocol=socks4&timeout=10000&country=all&ssl=all&anonymity=all", "json"},
		{"https://api.proxyscrape.com/v2/?request=get&protocol=socks5&timeout=10000&country=all&ssl=all&anonymity=all", "json"},
	}
}
