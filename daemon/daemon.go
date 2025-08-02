package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regproxy/api"
	"regproxy/config"
	"regproxy/crawler"
	"regproxy/storage"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Daemon represents the proxy testing daemon
type Daemon struct {
	config          *config.Config
	crawler         *crawler.Crawler
	tester          *api.ElevenLabsTester
	mongoStorage    *storage.MongoStorage
	workingProxies  []string
	logger          *log.Logger
	lastCrawlTime   time.Time
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewDaemon creates a new daemon instance
func NewDaemon(cfg *config.Config) (*Daemon, error) {
	// Setup logger
	var logger *log.Logger
	if cfg.Files.LogFile != "" {
		logFile, err := os.OpenFile(cfg.Files.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("error opening log file: %v", err)
		}
		logger = log.New(logFile, "[RegProxy] ", log.LstdFlags)
	} else {
		logger = log.New(os.Stdout, "[RegProxy] ", log.LstdFlags)
	}

	// Create crawler
	proxyCrawler := crawler.NewCrawler()
	proxyCrawler.SetMaxWorkers(cfg.Proxy.MaxCrawlWorkers)
	proxyCrawler.SetTimeout(cfg.GetTimeout())

	// Create ElevenLabs tester
	elevenLabsTester := api.NewElevenLabsTester(
		cfg.API.ElevenLabs.Key,
		cfg.API.ElevenLabs.URL,
		cfg.API.ElevenLabs.TestPayload,
		cfg.GetTimeout(),
	)

	ctx, cancel := context.WithCancel(context.Background())

	daemon := &Daemon{
		config:  cfg,
		crawler: proxyCrawler,
		tester:  elevenLabsTester,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Initialize MongoDB storage if enabled
	if cfg.MongoDB.Enabled {
		mongoStorage, err := storage.NewMongoStorage(
			cfg.MongoDB.DSN,
			cfg.MongoDB.Database,
			cfg.MongoDB.Collection,
			cfg.GetMongoTimeout(),
			logger,
		)
		if err != nil {
			logger.Printf("Warning: Failed to connect to MongoDB: %v", err)
			logger.Println("Continuing without MongoDB storage...")
		} else {
			daemon.mongoStorage = mongoStorage
			logger.Println("MongoDB storage enabled")
		}
	}

	// Load existing working proxies if available
	if err := daemon.loadWorkingProxies(); err != nil {
		logger.Printf("Warning: Could not load existing working proxies: %v", err)
	}

	return daemon, nil
}

// Run starts the daemon
func (d *Daemon) Run() error {
	d.logger.Println("🚀 RegProxy daemon starting...")
	
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initial proxy crawling if needed
	if len(d.workingProxies) == 0 {
		d.logger.Println("No working proxies found, performing initial crawl...")
		if err := d.crawlAndTestProxies(); err != nil {
			d.logger.Printf("Error in initial crawl: %v", err)
		}
	}

	// Main daemon loop
	ticker := time.NewTicker(d.config.GetInterval())
	defer ticker.Stop()

	crawlTicker := time.NewTicker(d.config.GetSourcesRefreshInterval())
	defer crawlTicker.Stop()

	d.logger.Printf("Daemon running with %d working proxies. Testing every %v", 
		len(d.workingProxies), d.config.GetInterval())

	for {
		select {
		case <-sigChan:
			d.logger.Println("Received shutdown signal, stopping daemon...")
			d.cancel()
			return d.shutdown()

		case <-ticker.C:
			d.logger.Println("Starting proxy test cycle...")
			if err := d.testExistingProxies(); err != nil {
				d.logger.Printf("Error testing proxies: %v", err)
			}

		case <-crawlTicker.C:
			d.logger.Println("Starting proxy crawl cycle...")
			if err := d.crawlAndTestProxies(); err != nil {
				d.logger.Printf("Error crawling proxies: %v", err)
			}

		case <-d.ctx.Done():
			return nil
		}
	}
}

// crawlAndTestProxies crawls new proxies and tests them
func (d *Daemon) crawlAndTestProxies() error {
	start := time.Now()
	d.logger.Println("Crawling proxies from sources...")

	// Crawl proxies
	crawlCtx, cancel := context.WithTimeout(d.ctx, 5*time.Minute)
	defer cancel()

	proxies, err := d.crawler.CrawlProxies(crawlCtx)
	if err != nil {
		return fmt.Errorf("error crawling proxies: %v", err)
	}

	d.logger.Printf("Crawled %d proxies in %v", len(proxies), time.Since(start))

	// Save all proxies
	if err := d.crawler.SaveToFile(proxies, d.config.Files.AllProxies); err != nil {
		d.logger.Printf("Warning: Could not save all proxies: %v", err)
	}

	// Test a sample of proxies
	sampleSize := d.config.Proxy.TestSampleSize
	if len(proxies) < sampleSize {
		sampleSize = len(proxies)
	}

	testSample := proxies[:sampleSize]
	return d.testProxies(testSample, "crawl")
}

// testExistingProxies tests the current working proxies
func (d *Daemon) testExistingProxies() error {
	if len(d.workingProxies) == 0 {
		d.logger.Println("No working proxies to test, performing crawl...")
		return d.crawlAndTestProxies()
	}

	return d.testProxies(d.workingProxies, "maintenance")
}

// testProxies tests a list of proxies against ElevenLabs API
func (d *Daemon) testProxies(proxies []string, testType string) error {
	if len(proxies) == 0 {
		return nil
	}

	start := time.Now()
	d.logger.Printf("Testing %d proxies (%s)...", len(proxies), testType)

	// Test proxies
	testCtx, cancel := context.WithTimeout(d.ctx, 10*time.Minute)
	defer cancel()

	results := d.tester.TestProxies(testCtx, proxies, d.config.Daemon.Threads)
	
	// Get working proxies
	newWorkingProxies := api.GetWorkingProxies(results)
	
	// Sort by latency for better performance
	sort.Strings(newWorkingProxies)

	// Keep only the best proxies
	if len(newWorkingProxies) > d.config.Proxy.KeepWorkingProxies {
		newWorkingProxies = newWorkingProxies[:d.config.Proxy.KeepWorkingProxies]
	}

	// Update working proxies
	d.workingProxies = newWorkingProxies

	// Save to MongoDB if enabled
	if d.mongoStorage != nil {
		storageResults := d.convertToStorageResults(results)
		if err := d.mongoStorage.SaveWorkingProxies(testCtx, storageResults); err != nil {
			d.logger.Printf("Warning: Failed to save to MongoDB: %v", err)
		}
	}

	// Save working proxies to file
	if err := d.saveWorkingProxies(); err != nil {
		d.logger.Printf("Warning: Could not save working proxies: %v", err)
	}

	d.logger.Printf("Test completed in %v. Working proxies: %d/%d (%.2f%%)", 
		time.Since(start), len(newWorkingProxies), len(results), 
		float64(len(newWorkingProxies))/float64(len(results))*100)

	// Log sample of working proxies
	sampleSize := 3
	if len(newWorkingProxies) < sampleSize {
		sampleSize = len(newWorkingProxies)
	}
	for i := 0; i < sampleSize; i++ {
		d.logger.Printf("Working proxy: %s", newWorkingProxies[i])
	}

	return nil
}

// convertToStorageResults converts API test results to storage format
func (d *Daemon) convertToStorageResults(apiResults []api.TestResult) []storage.ProxyTestResult {
	storageResults := make([]storage.ProxyTestResult, len(apiResults))
	
	for i, apiResult := range apiResults {
		parts := strings.Split(apiResult.Proxy, ":")
		ip := apiResult.Proxy
		port := ""
		if len(parts) == 2 {
			ip = parts[0]
			port = parts[1]
		}
		
		storageResults[i] = storage.ProxyTestResult{
			Address:   apiResult.Proxy,
			IP:        ip,
			Port:      port,
			Type:      "http", // Default to HTTP, could be enhanced to detect type
			IsWorking: apiResult.IsWorking,
			Latency:   apiResult.Latency,
			Error:     apiResult.Error,
		}
	}
	
	return storageResults
}

// saveWorkingProxies saves working proxies to file
func (d *Daemon) saveWorkingProxies() error {
	return d.crawler.SaveToFile(d.workingProxies, d.config.Files.WorkingProxies)
}

// loadWorkingProxies loads working proxies from file
func (d *Daemon) loadWorkingProxies() error {
	// Try to load from MongoDB first if enabled
	if d.mongoStorage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		mongoProxies, err := d.mongoStorage.GetWorkingProxies(ctx, d.config.Proxy.KeepWorkingProxies)
		if err != nil {
			d.logger.Printf("Warning: Could not load proxies from MongoDB: %v", err)
		} else if len(mongoProxies) > 0 {
			d.workingProxies = mongoProxies
			d.logger.Printf("Loaded %d working proxies from MongoDB", len(mongoProxies))
			return nil
		}
	}

	// Fallback to file
	proxies, err := d.crawler.LoadFromFile(d.config.Files.WorkingProxies)
	if err != nil {
		return err
	}
	d.workingProxies = proxies
	d.logger.Printf("Loaded %d working proxies from file", len(proxies))
	return nil
}

// GetWorkingProxies returns the current list of working proxies
func (d *Daemon) GetWorkingProxies() []string {
	return d.workingProxies
}

// GetStats returns daemon statistics
func (d *Daemon) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"working_proxies": len(d.workingProxies),
		"last_crawl":      d.lastCrawlTime,
		"uptime":          time.Since(d.lastCrawlTime),
		"mongodb_enabled": d.mongoStorage != nil,
	}

	// Add MongoDB stats if available
	if d.mongoStorage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		mongoStats, err := d.mongoStorage.GetProxyStats(ctx)
		if err != nil {
			d.logger.Printf("Warning: Could not get MongoDB stats: %v", err)
		} else {
			stats["mongodb_stats"] = mongoStats
		}
	}

	return stats
}

// shutdown gracefully shuts down the daemon
func (d *Daemon) shutdown() error {
	d.logger.Println("Shutting down daemon...")
	
	// Save current working proxies
	if err := d.saveWorkingProxies(); err != nil {
		d.logger.Printf("Error saving working proxies during shutdown: %v", err)
	}

	// Close MongoDB connection
	if d.mongoStorage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := d.mongoStorage.Close(ctx); err != nil {
			d.logger.Printf("Error closing MongoDB connection: %v", err)
		} else {
			d.logger.Println("MongoDB connection closed")
		}
	}

	d.logger.Println("Daemon stopped")
	return nil
}
