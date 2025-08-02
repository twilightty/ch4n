package daemon

import (
	"context"
	"fmt"
	stdlog "log"
	"os"
	"os/signal"
	"regproxy/api"
	"regproxy/config"
	"regproxy/crawler"
	"regproxy/logger"
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
	logger          *logger.Logger
	lastCrawlTime   time.Time
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewDaemon creates a new daemon instance
func NewDaemon(cfg *config.Config) (*Daemon, error) {
	// Setup logger
	log, err := logger.NewLogger(cfg.Daemon.LogLevel, cfg.Files.LogFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %v", err)
	}

	// Create crawler
	proxyCrawler := crawler.NewCrawler()
	proxyCrawler.SetMaxWorkers(cfg.Proxy.MaxCrawlWorkers)
	proxyCrawler.SetTimeout(cfg.GetTimeout())

	// Create ElevenLabs tester
	tester := api.NewElevenLabsTester(cfg.API.ElevenLabs.Key, cfg.API.ElevenLabs.URL, cfg.API.ElevenLabs.TestPayload, cfg.GetTimeout())

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	daemon := &Daemon{
		config:  cfg,
		crawler: proxyCrawler,
		tester:  tester,
		logger:  log,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Initialize MongoDB if enabled
	if cfg.MongoDB.Enabled {
		// Convert our logger to standard log.Logger for MongoDB storage
		stdLogger := stdlog.New(os.Stdout, "[MongoDB] ", stdlog.LstdFlags)
		mongoStorage, err := storage.NewMongoStorage(cfg.MongoDB.DSN, cfg.MongoDB.Database, cfg.MongoDB.Collection, cfg.GetMongoTimeout(), stdLogger)
		if err != nil {
			log.Warn("Failed to connect to MongoDB: %v", err)
			log.Warn("Continuing without MongoDB storage...")
		} else {
			daemon.mongoStorage = mongoStorage
			log.Info("MongoDB storage enabled")
		}
	}

	// Load existing working proxies
	if err := daemon.loadWorkingProxies(); err != nil {
		log.Warn("Could not load existing working proxies: %v", err)
	}

	return daemon, nil
}

// Run starts the daemon
func (d *Daemon) Run() error {
	d.logger.Info("🚀 RegProxy daemon starting...")
	
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initial proxy crawling if needed
	if len(d.workingProxies) == 0 {
		d.logger.Info("No working proxies found, performing initial crawl...")
		if err := d.crawlAndTestProxies(); err != nil {
			d.logger.Info("Error in initial crawl: %v", err)
		}
	}

	// Main daemon loop
	ticker := time.NewTicker(d.config.GetInterval())
	defer ticker.Stop()

	crawlTicker := time.NewTicker(d.config.GetSourcesRefreshInterval())
	defer crawlTicker.Stop()

	d.logger.Info("Daemon running with %d working proxies. Testing every %v", 
		len(d.workingProxies), d.config.GetInterval())

	for {
		select {
		case <-sigChan:
			d.logger.Info("Received shutdown signal, stopping daemon...")
			d.cancel()
			return d.shutdown()

		case <-ticker.C:
			d.logger.Info("Starting proxy test cycle...")
			if err := d.testExistingProxies(); err != nil {
				d.logger.Info("Error testing proxies: %v", err)
			}

		case <-crawlTicker.C:
			d.logger.Info("Starting proxy crawl cycle...")
			if err := d.crawlAndTestProxies(); err != nil {
				d.logger.Info("Error crawling proxies: %v", err)
			}

		case <-d.ctx.Done():
			return nil
		}
	}
}

// crawlAndTestProxies crawls new proxies and tests them
func (d *Daemon) crawlAndTestProxies() error {
	start := time.Now()
	d.logger.Info("Crawling proxies from sources...")

	// Crawl proxies
	crawlCtx, cancel := context.WithTimeout(d.ctx, 5*time.Minute)
	defer cancel()

	proxies, err := d.crawler.CrawlProxies(crawlCtx)
	if err != nil {
		return fmt.Errorf("error crawling proxies: %v", err)
	}

	d.logger.Info("Crawled %d proxies in %v", len(proxies), time.Since(start))

	// Save all proxies
	if err := d.crawler.SaveToFile(proxies, d.config.Files.AllProxies); err != nil {
		d.logger.Info("Warning: Could not save all proxies: %v", err)
	}

	// Test proxies - use all if sample size is -1 or 0, otherwise use sample
	var testSample []string
	sampleSize := d.config.Proxy.TestSampleSize
	
	if sampleSize <= 0 {
		// Test all proxies
		testSample = proxies
		d.logger.Info("Testing ALL %d proxies (sample size disabled)", len(proxies))
	} else {
		// Test sample
		if len(proxies) < sampleSize {
			sampleSize = len(proxies)
		}
		testSample = proxies[:sampleSize]
		d.logger.Info("Testing %d out of %d proxies (sample)", sampleSize, len(proxies))
	}

	return d.testProxies(testSample, "crawl")
}

// testExistingProxies tests the current working proxies
func (d *Daemon) testExistingProxies() error {
	if len(d.workingProxies) == 0 {
		d.logger.Info("No working proxies to test, performing crawl...")
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
	d.logger.Info("Testing %d proxies (%s)...", len(proxies), testType)
	
	// Show progress for large batches
	if len(proxies) > 1000 {
		d.logger.Info("⏳ This is a large batch - testing %d proxies may take %d+ minutes...", 
			len(proxies), len(proxies)/(d.config.Daemon.Threads*3)) // Rough estimate: 3 proxies per second per thread
	}

	// Test proxies
	testCtx, cancel := context.WithTimeout(d.ctx, 10*time.Minute)
	defer cancel()

	results := d.tester.TestProxies(testCtx, proxies, d.config.Daemon.Threads)
	
	// Process results and save working proxies immediately in batches
	var workingProxies []string
	var batchResults []storage.ProxyTestResult
	batchSize := 10 // Save every 10 working proxies
	
	successCount := 0
	for i, result := range results {
		if result.IsWorking {
			successCount++
			workingProxies = append(workingProxies, result.Proxy)
			d.logger.Info("✅ WORKING: %s (latency: %dms)", result.Proxy, result.Latency.Milliseconds())
			
			// Prepare for MongoDB storage
			if d.mongoStorage != nil {
				parts := strings.Split(result.Proxy, ":")
				ip := parts[0]
				port := ""
				if len(parts) > 1 {
					port = parts[1]
				}
				
				storageResult := storage.ProxyTestResult{
					Address:   result.Proxy,
					IP:        ip,
					Port:      port,
					Type:      "http",
					IsWorking: result.IsWorking,
					Latency:   result.Latency,
					Error:     result.Error,
				}
				batchResults = append(batchResults, storageResult)
				
				// Save batch when we have enough working proxies OR at the end
				if len(batchResults) >= batchSize || i == len(results)-1 {
					if len(batchResults) > 0 {
						d.logger.Info("💾 Saving batch of %d working proxies to MongoDB...", len(batchResults))
						if err := d.mongoStorage.SaveWorkingProxies(testCtx, batchResults); err != nil {
							d.logger.Error("Failed to save batch to MongoDB: %v", err)
						} else {
							d.logger.Info("✅ Saved batch of %d working proxies to MongoDB", len(batchResults))
						}
						batchResults = nil // Reset batch
					}
				}
			}
		} else {
			errorMsg := "unknown error"
			if result.Error != nil {
				errorMsg = result.Error.Error()
			}
			d.logger.Debug("❌ FAILED: %s (error: %s)", result.Proxy, errorMsg)
		}
	}
	
	// Sort working proxies by performance
	sort.Strings(workingProxies)

	// Keep only the best proxies
	if len(workingProxies) > d.config.Proxy.KeepWorkingProxies {
		workingProxies = workingProxies[:d.config.Proxy.KeepWorkingProxies]
	}

	// Update working proxies in memory
	d.workingProxies = workingProxies

	// Save working proxies to file
	if err := d.saveWorkingProxies(); err != nil {
		d.logger.Error("Could not save working proxies to file: %v", err)
	}

	successRate := float64(successCount) / float64(len(results)) * 100
	d.logger.Info("📊 Test completed in %v. Working: %d/%d (%.2f%%)", 
		time.Since(start), successCount, len(results), successRate)

	// Log sample of working proxies
	sampleSize := 5
	if len(workingProxies) < sampleSize {
		sampleSize = len(workingProxies)
	}
	
	if sampleSize > 0 {
		d.logger.Info("📋 Sample working proxies:")
		for i := 0; i < sampleSize; i++ {
			d.logger.Info("   %d. %s", i+1, workingProxies[i])
		}
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
			d.logger.Info("Warning: Could not load proxies from MongoDB: %v", err)
		} else if len(mongoProxies) > 0 {
			d.workingProxies = mongoProxies
			d.logger.Info("Loaded %d working proxies from MongoDB", len(mongoProxies))
			return nil
		}
	}

	// Fallback to file
	proxies, err := d.crawler.LoadFromFile(d.config.Files.WorkingProxies)
	if err != nil {
		return err
	}
	d.workingProxies = proxies
	d.logger.Info("Loaded %d working proxies from file", len(proxies))
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
			d.logger.Info("Warning: Could not get MongoDB stats: %v", err)
		} else {
			stats["mongodb_stats"] = mongoStats
		}
	}

	return stats
}

// shutdown gracefully shuts down the daemon
func (d *Daemon) shutdown() error {
	d.logger.Info("Shutting down daemon...")
	
	// Save current working proxies
	if err := d.saveWorkingProxies(); err != nil {
		d.logger.Info("Error saving working proxies during shutdown: %v", err)
	}

	// Close MongoDB connection
	if d.mongoStorage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := d.mongoStorage.Close(ctx); err != nil {
			d.logger.Info("Error closing MongoDB connection: %v", err)
		} else {
			d.logger.Info("MongoDB connection closed")
		}
	}

	d.logger.Info("Daemon stopped")
	return nil
}
