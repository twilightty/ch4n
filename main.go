package main

import (
	"flag"
	"fmt"
	"log"
	"regproxy/config"
	"regproxy/daemon"
)

func main() {
	// Define command line flags
	var (
		configFile = flag.String("config", "config.yaml", "Path to configuration file")
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

	fmt.Println("🚀 RegProxy - Proxy Testing Daemon")
	fmt.Println("===================================")
	fmt.Printf("Configuration loaded from: %s\n", *configFile)
	fmt.Printf("ElevenLabs API URL: %s\n", cfg.API.ElevenLabs.URL)
	fmt.Printf("Test interval: %v\n", cfg.GetInterval())
	fmt.Printf("Threads: %d\n", cfg.Daemon.Threads)
	fmt.Printf("Timeout: %v\n", cfg.GetTimeout())
	fmt.Println()

	// Create and start daemon
	d, err := daemon.NewDaemon(cfg)
	if err != nil {
		log.Fatalf("Error creating daemon: %v", err)
	}

	// Run daemon
	if err := d.Run(); err != nil {
		log.Fatalf("Daemon error: %v", err)
	}
}

func showHelp() {
	fmt.Println("RegProxy - Proxy Testing Daemon")
	fmt.Println("================================")
	fmt.Println()
	fmt.Println("This daemon continuously tests proxies against the ElevenLabs API")
	fmt.Println("to maintain a list of working proxies.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  regproxy [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  Create a config.yaml file to customize settings.")
	fmt.Println("  The daemon will use default values if no config file is found.")
	fmt.Println()
	fmt.Println("Example config.yaml:")
	fmt.Println(`  api:
    elevenlabs:
      key: "your-api-key-here"
      url: "https://api.elevenlabs.io/v1/text-to-speech/..."
  
  daemon:
    interval: 300      # test interval in seconds
    threads: 20        # concurrent threads
    timeout: 10        # request timeout in seconds
  
  proxy:
    test_sample_size: 100     # proxies to test each cycle
    keep_working_proxies: 50  # max working proxies to keep`)
	fmt.Println()
	fmt.Println("The daemon will:")
	fmt.Println("  - Crawl proxies from multiple sources")
	fmt.Println("  - Test them against ElevenLabs API")
	fmt.Println("  - Maintain a list of working proxies")
	fmt.Println("  - Re-test proxies periodically")
	fmt.Println("  - Save working proxies to working_proxies.txt")
}
