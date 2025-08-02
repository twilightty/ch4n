package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	API struct {
		ElevenLabs struct {
			Key         string `yaml:"key"`
			URL         string `yaml:"url"`
			TestPayload string `yaml:"test_payload"`
		} `yaml:"elevenlabs"`
	} `yaml:"api"`

	MongoDB struct {
		Enabled    bool   `yaml:"enabled"`
		DSN        string `yaml:"dsn"`
		Database   string `yaml:"database"`
		Collection string `yaml:"collection"`
		Timeout    int    `yaml:"timeout"`
	} `yaml:"mongodb"`

	Daemon struct {
		Interval  int    `yaml:"interval"`
		Threads   int    `yaml:"threads"`
		Timeout   int    `yaml:"timeout"`
		LogLevel  string `yaml:"log_level"`
	} `yaml:"daemon"`

	Proxy struct {
		SourcesRefreshInterval int `yaml:"sources_refresh_interval"`
		MaxCrawlWorkers        int `yaml:"max_crawl_workers"`
		TestSampleSize         int `yaml:"test_sample_size"`
		KeepWorkingProxies     int `yaml:"keep_working_proxies"`
	} `yaml:"proxy"`

	Files struct {
		WorkingProxies string `yaml:"working_proxies"`
		AllProxies     string `yaml:"all_proxies"`
		LogFile        string `yaml:"log_file"`
	} `yaml:"files"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}

	// Set default values
	config.Daemon.Interval = 300
	config.Daemon.Threads = 20
	config.Daemon.Timeout = 10
	config.Daemon.LogLevel = "info"
	config.Proxy.SourcesRefreshInterval = 3600
	config.Proxy.MaxCrawlWorkers = 15
	config.Proxy.TestSampleSize = 100
	config.Proxy.KeepWorkingProxies = 50
	config.Files.WorkingProxies = "working_proxies.txt"
	config.Files.AllProxies = "proxies.txt"
	config.Files.LogFile = "daemon.log"
	config.API.ElevenLabs.URL = "https://api.elevenlabs.io/v1/text-to-speech/JBFqnCBsd6RMkjVDRZzb?output_format=mp3_44100_128"
	config.API.ElevenLabs.TestPayload = `{"text": "The first move is what sets everything in motion.", "model_id": "eleven_multilingual_v2"}`
	config.MongoDB.Enabled = false
	config.MongoDB.DSN = "mongodb://localhost:27017"
	config.MongoDB.Database = "regproxy"
	config.MongoDB.Collection = "proxy"
	config.MongoDB.Timeout = 10

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("Config file not found, using defaults. Create %s to customize settings.\n", configPath)
		return config, nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("error opening config file: %v", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Validate required fields
	if config.API.ElevenLabs.Key == "" || config.API.ElevenLabs.Key == "your-elevenlabs-api-key-here" {
		return nil, fmt.Errorf("please set your ElevenLabs API key in the config file")
	}

	return config, nil
}

// GetInterval returns the daemon interval as time.Duration
func (c *Config) GetInterval() time.Duration {
	return time.Duration(c.Daemon.Interval) * time.Second
}

// GetTimeout returns the request timeout as time.Duration
func (c *Config) GetTimeout() time.Duration {
	return time.Duration(c.Daemon.Timeout) * time.Second
}

// GetSourcesRefreshInterval returns the sources refresh interval as time.Duration
func (c *Config) GetSourcesRefreshInterval() time.Duration {
	return time.Duration(c.Proxy.SourcesRefreshInterval) * time.Second
}

// GetMongoTimeout returns the MongoDB connection timeout as time.Duration
func (c *Config) GetMongoTimeout() time.Duration {
	return time.Duration(c.MongoDB.Timeout) * time.Second
}
