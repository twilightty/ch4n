package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"regproxy/api"
	"regproxy/config"
	"regproxy/crawler"
	"time"
)

func main() {
	var (
		configFile = flag.String("config", "config.yaml", "Path to configuration file")
		action     = flag.String("action", "test", "Action to perform: test, crawl, validate")
		proxyFile  = flag.String("file", "working_proxies.txt", "Proxy file to use")
		count      = flag.Int("count", 10, "Number of proxies to test")
		help       = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	switch *action {
	case "test":
		testProxies(cfg, *proxyFile, *count)
	case "crawl":
		crawlProxies(cfg)
	case "validate":
		validateConfig(cfg)
	default:
		fmt.Printf("Unknown action: %s\n", *action)
		showHelp()
	}
}

func testProxies(cfg *config.Config, proxyFile string, count int) {
	fmt.Printf("🔍 Testing proxies from %s...\n", proxyFile)

	// Load proxies
	crawler := crawler.NewCrawler()
	proxies, err := crawler.LoadFromFile(proxyFile)
	if err != nil {
		log.Fatalf("Error loading proxies: %v", err)
	}

	if len(proxies) == 0 {
		fmt.Println("No proxies found in file!")
		return
	}

	// Limit count
	if count > len(proxies) {
		count = len(proxies)
	}
	testProxies := proxies[:count]

	fmt.Printf("Testing %d proxies against ElevenLabs API...\n", len(testProxies))

	// Create tester
	tester := api.NewElevenLabsTester(
		cfg.API.ElevenLabs.Key,
		cfg.API.ElevenLabs.URL,
		cfg.API.ElevenLabs.TestPayload,
		cfg.GetTimeout(),
	)

	// Test proxies
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	results := tester.TestProxies(ctx, testProxies, cfg.Daemon.Threads)

	// Print results
	api.PrintResults(results, true)

	// Save working proxies
	workingProxies := api.GetWorkingProxies(results)
	if len(workingProxies) > 0 {
		if err := crawler.SaveToFile(workingProxies, "tested_working_proxies.txt"); err != nil {
			log.Printf("Error saving working proxies: %v", err)
		} else {
			fmt.Printf("✅ Working proxies saved to tested_working_proxies.txt\n")
		}
	}
}

func crawlProxies(cfg *config.Config) {
	fmt.Println("🚀 Crawling proxies from sources...")

	// Create crawler
	crawler := crawler.NewCrawler()
	crawler.SetMaxWorkers(cfg.Proxy.MaxCrawlWorkers)
	crawler.SetTimeout(cfg.GetTimeout())

	// Crawl proxies
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	proxies, err := crawler.CrawlProxies(ctx)
	if err != nil {
		log.Fatalf("Error crawling proxies: %v", err)
	}

	fmt.Printf("✅ Found %d proxies\n", len(proxies))

	// Save proxies
	if err := crawler.SaveToFile(proxies, "crawled_proxies.txt"); err != nil {
		log.Printf("Error saving proxies: %v", err)
	} else {
		fmt.Println("✅ Proxies saved to crawled_proxies.txt")
	}

	// Show sample
	samples := crawler.GetSampleProxies(proxies, 5)
	fmt.Printf("\n📋 Sample proxies:\n")
	for i, proxy := range samples {
		fmt.Printf("   %d. %s\n", i+1, proxy)
	}
}

func validateConfig(cfg *config.Config) {
	fmt.Println("🔍 Validating configuration...")

	fmt.Printf("✅ ElevenLabs API Key: %s\n", maskAPIKey(cfg.API.ElevenLabs.Key))
	fmt.Printf("✅ API URL: %s\n", cfg.API.ElevenLabs.URL)
	fmt.Printf("✅ Test interval: %v\n", cfg.GetInterval())
	fmt.Printf("✅ Threads: %d\n", cfg.Daemon.Threads)
	fmt.Printf("✅ Timeout: %v\n", cfg.GetTimeout())
	fmt.Printf("✅ Test sample size: %d\n", cfg.Proxy.TestSampleSize)
	fmt.Printf("✅ Keep working proxies: %d\n", cfg.Proxy.KeepWorkingProxies)

	// MongoDB configuration
	fmt.Printf("\n🗄️ MongoDB Configuration:\n")
	fmt.Printf("   Enabled: %v\n", cfg.MongoDB.Enabled)
	if cfg.MongoDB.Enabled {
		fmt.Printf("   DSN: %s\n", cfg.MongoDB.DSN)
		fmt.Printf("   Database: %s\n", cfg.MongoDB.Database)
		fmt.Printf("   Collection: %s\n", cfg.MongoDB.Collection)
		fmt.Printf("   Timeout: %v\n", cfg.GetMongoTimeout())
	}

	// Test API connection (basic validation)
	fmt.Println("\n🔍 Testing ElevenLabs API configuration...")
	
	if cfg.API.ElevenLabs.Key == "" {
		fmt.Println("❌ ElevenLabs API key is not set!")
		return
	}
	
	if cfg.API.ElevenLabs.URL == "" {
		fmt.Println("❌ ElevenLabs API URL is not set!")
		return
	}

	fmt.Println("✅ Configuration is valid!")
	fmt.Println("\nNote: Run the daemon to test actual API connectivity with proxies.")
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func showHelp() {
	fmt.Println("RegProxy CLI - Proxy Management Tool")
	fmt.Println("=====================================")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  regproxy-cli [flags]")
	fmt.Println()
	fmt.Println("Actions:")
	fmt.Println("  test     - Test proxies against ElevenLabs API")
	fmt.Println("  crawl    - Crawl new proxies from sources")
	fmt.Println("  validate - Validate configuration")
	fmt.Println()
	fmt.Println("Flags:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Test 20 proxies from working_proxies.txt")
	fmt.Println("  regproxy-cli -action test -count 20")
	fmt.Println()
	fmt.Println("  # Crawl new proxies")
	fmt.Println("  regproxy-cli -action crawl")
	fmt.Println()
	fmt.Println("  # Validate configuration")
	fmt.Println("  regproxy-cli -action validate")
}
