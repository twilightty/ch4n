package main

import (
	"context"
	"fmt"
	"regproxy/crawler"
	"time"
)

func main() {
	fmt.Println("🧪 Testing proxy crawler...")
	
	c := crawler.NewCrawler()
	c.SetMaxWorkers(5)
	c.SetTimeout(10 * time.Second)
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	proxies, err := c.CrawlProxies(ctx)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	
	fmt.Printf("✅ Success! Found %d proxies\n", len(proxies))
	
	// Show first 10 proxies
	sample := 10
	if len(proxies) < sample {
		sample = len(proxies)
	}
	
	fmt.Printf("\n📋 First %d proxies:\n", sample)
	for i := 0; i < sample; i++ {
		fmt.Printf("%d. %s\n", i+1, proxies[i])
	}
}
