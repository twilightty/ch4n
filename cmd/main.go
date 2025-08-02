package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"regproxy/crawler"
	"time"
)

func main() {
	// Define command line flags
	var (
		workers     = flag.Int("workers", 15, "Number of concurrent workers for crawling")
		timeout     = flag.Int("timeout", 10, "Timeout in seconds for HTTP requests")
		output      = flag.String("output", "proxies.txt", "Output file for proxies")
		test        = flag.Bool("test", false, "Test proxies after crawling")
		testWorkers = flag.Int("test-workers", 20, "Number of concurrent workers for testing")
		testTimeout = flag.Int("test-timeout", 5, "Timeout in seconds for proxy testing")
		testSample  = flag.Int("test-sample", 50, "Number of proxies to test (0 for all)")
		load        = flag.String("load", "", "Load proxies from file instead of crawling")
		help        = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// Load proxies from file if specified
	if *load != "" {
		loadAndProcessProxies(*load, *test, *testWorkers, *testTimeout, *output)
		return
	}

	// Create a new crawler
	proxyCrawler := crawler.NewCrawler()

	// Set crawler options
	proxyCrawler.SetMaxWorkers(*workers)
	proxyCrawler.SetTimeout(time.Duration(*timeout) * time.Second)

	// Create context with timeout (5 minutes for crawling)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Println("🚀 RegProxy - Go Proxy Crawler")
	fmt.Println("===============================")

	// Crawl proxies
	proxies, err := proxyCrawler.CrawlProxies(ctx)
	if err != nil {
		log.Fatalf("Error crawling proxies: %v", err)
	}

	if len(proxies) == 0 {
		fmt.Println("❌ No proxies found!")
		return
	}

	// Save proxies to file
	if err := proxyCrawler.SaveToFile(proxies, *output); err != nil {
		log.Printf("Error saving proxies: %v", err)
	} else {
		fmt.Printf("✅ Proxies saved to %s\n", *output)
	}

	// Show sample proxies
	fmt.Printf("\n📋 Sample proxies:\n")
	samples := proxyCrawler.GetSampleProxies(proxies, 5)
	for i, proxy := range samples {
		fmt.Printf("   %d. %s\n", i+1, proxy)
	}

	// Create proxy manager for statistics
	manager := crawler.NewProxyManager()
	manager.AddProxies(proxies, crawler.HTTP) // Assume HTTP for demo
	manager.PrintStats()

	// Test proxies if requested
	if *test {
		testProxiesFn(ctx, proxies, *testWorkers, *testTimeout, *testSample, *output)
	}

	fmt.Println("\n🎉 Proxy crawling completed!")
}

func testProxiesFn(ctx context.Context, proxies []string, workers, timeoutSec, sampleSize int, outputPrefix string) {
	fmt.Println("\n🔍 Testing proxies...")

	tester := crawler.NewProxyTester()
	tester.SetMaxWorkers(workers)
	tester.SetTimeout(time.Duration(timeoutSec) * time.Second)

	// Determine test sample
	testSample := proxies
	if sampleSize > 0 && len(proxies) > sampleSize {
		testSample = proxies[:sampleSize]
		fmt.Printf("Testing sample of %d proxies...\n", sampleSize)
	}

	workingProxies, err := tester.TestProxies(ctx, testSample)
	if err != nil {
		log.Printf("Error testing proxies: %v", err)
		return
	}

	fmt.Printf("✅ Found %d working proxies out of %d tested\n", len(workingProxies), len(testSample))

	// Save working proxies
	if len(workingProxies) > 0 {
		workingFile := "working_" + outputPrefix
		crawler := crawler.NewCrawler()
		if err := crawler.SaveToFile(workingProxies, workingFile); err != nil {
			log.Printf("Error saving working proxies: %v", err)
		} else {
			fmt.Printf("✅ Working proxies saved to %s\n", workingFile)
		}

		// Show sample working proxies
		fmt.Printf("\n📋 Sample working proxies:\n")
		sampleCount := 3
		if len(workingProxies) < sampleCount {
			sampleCount = len(workingProxies)
		}
		for i := 0; i < sampleCount; i++ {
			fmt.Printf("   %d. %s\n", i+1, workingProxies[i])
		}
	}
}

func loadAndProcessProxies(filename string, test bool, testWorkers, testTimeout int, output string) {
	fmt.Printf("📂 Loading proxies from %s...\n", filename)

	crawler := crawler.NewCrawler()
	proxies, err := crawler.LoadFromFile(filename)
	if err != nil {
		log.Fatalf("Error loading proxies: %v", err)
	}

	fmt.Printf("✅ Loaded %d proxies from file\n", len(proxies))

	// Create proxy manager for statistics
	manager := crawler.NewProxyManager()
	manager.AddProxies(proxies, crawler.HTTP)
	manager.PrintStats()

	if test {
		ctx := context.Background()
		testProxiesFn(ctx, proxies, testWorkers, testTimeout, 0, output)
	}
}

func showHelp() {
	fmt.Println("RegProxy - Go Proxy Crawler")
	fmt.Println("============================")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  regproxy [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Basic crawling")
	fmt.Println("  regproxy")
	fmt.Println()
	fmt.Println("  # Crawl with custom settings")
	fmt.Println("  regproxy -workers 20 -timeout 15 -output my_proxies.txt")
	fmt.Println()
	fmt.Println("  # Crawl and test proxies")
	fmt.Println("  regproxy -test -test-sample 100")
	fmt.Println()
	fmt.Println("  # Load and test existing proxies")
	fmt.Println("  regproxy -load proxies.txt -test")
	fmt.Println()
	fmt.Println("  # Test only a few proxies")
	fmt.Println("  regproxy -test -test-sample 20 -test-workers 50")
}
