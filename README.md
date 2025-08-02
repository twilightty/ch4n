# RegProxy - Go Proxy Crawler and Testing Daemon

RegProxy is a Go-based proxy crawler and testing daemon that continuously finds and validates proxies against the ElevenLabs API. It maintains a pool of working proxies and runs as a background service.

## Features

- 🚀 **Automated Proxy Crawling**: Fetches proxies from 100+ sources
- 🔍 **ElevenLabs API Testing**: Tests proxies against real ElevenLabs API endpoints
- ⚡ **Multi-threaded Processing**: Configurable concurrent workers
- 🔄 **Continuous Operation**: Runs as a daemon with periodic testing
- 📊 **Statistics & Monitoring**: Detailed logging and statistics
- 💾 **Persistent Storage**: Saves working proxies to files and MongoDB
- 🗄️ **MongoDB Integration**: Store proxy data with rich metadata and statistics
- ⚙️ **Configurable**: YAML-based configuration

## Architecture

```
RegProxy/
├── main.go                 # Main daemon application
├── config.yaml            # Configuration file
├── crawler/               # Proxy crawling modules
│   ├── crawler.go         # Main crawler logic
│   ├── tester.go          # Proxy testing
│   └── manager.go         # Proxy management
├── api/                   # API testing modules
│   └── elevenlabs.go      # ElevenLabs API testing
├── daemon/                # Daemon functionality
│   └── daemon.go          # Main daemon logic
├── config/                # Configuration handling
│   └── config.go          # Config loading and validation
└── cmd/                   # Command-line tools
    └── cli/
        └── main.go        # CLI management tool
```

## Installation

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd RegProxy
   ```

2. **Build the applications:**
   ```bash
   go mod tidy
   go build -o regproxy-daemon main.go
   go build -o regproxy-cli cmd/cli/main.go
   ```

3. **Create configuration file:**
   ```bash
   cp config.yaml.example config.yaml
   # Edit config.yaml with your ElevenLabs API key
   ```

## Configuration

Create a `config.yaml` file with your settings:

```yaml
api:
  elevenlabs:
    key: "your-elevenlabs-api-key-here"
    url: "https://api.elevenlabs.io/v1/text-to-speech/JBFqnCBsd6RMkjVDRZzb?output_format=mp3_44100_128"
    test_payload: |
      {
        "text": "The first move is what sets everything in motion.",
        "model_id": "eleven_multilingual_v2"
      }

mongodb:
  enabled: true                    # Enable MongoDB storage
  dsn: "mongodb://localhost:27017" # MongoDB connection string
  database: "regproxy"            # Database name
  collection: "proxy"             # Collection name
  timeout: 10                     # Connection timeout in seconds

daemon:
  interval: 300           # seconds between proxy tests
  threads: 20             # concurrent threads for testing
  timeout: 10             # timeout in seconds for requests
  log_level: "info"

proxy:
  sources_refresh_interval: 3600    # seconds between crawling new proxies
  max_crawl_workers: 15             # concurrent workers for crawling
  test_sample_size: 100             # number of proxies to test each cycle
  keep_working_proxies: 50          # maximum working proxies to keep

files:
  working_proxies: "working_proxies.txt"
  all_proxies: "proxies.txt"
  log_file: "daemon.log"
```

### Configuration Options

- **api.elevenlabs.key**: Your ElevenLabs API key (required)
- **api.elevenlabs.url**: ElevenLabs API endpoint to test against
- **mongodb.enabled**: Enable/disable MongoDB storage (default: false)
- **mongodb.dsn**: MongoDB connection string (e.g., "mongodb://localhost:27017")
- **mongodb.database**: MongoDB database name
- **mongodb.collection**: MongoDB collection name for proxy data
- **mongodb.timeout**: MongoDB connection timeout in seconds
- **daemon.interval**: How often to test existing working proxies (seconds)
- **daemon.threads**: Number of concurrent threads for testing
- **daemon.timeout**: Request timeout for API calls
- **proxy.sources_refresh_interval**: How often to crawl new proxies (seconds)
- **proxy.test_sample_size**: Number of proxies to test in each cycle
- **proxy.keep_working_proxies**: Maximum number of working proxies to maintain

## Usage

### Running the Daemon

Start the daemon to continuously test proxies:

```bash
./regproxy-daemon
```

With custom config file:
```bash
./regproxy-daemon -config /path/to/config.yaml
```

The daemon will:
1. Load existing working proxies (if any)
2. If no working proxies exist, perform initial crawl
3. Test proxies against ElevenLabs API every `interval` seconds
4. Crawl new proxies every `sources_refresh_interval` seconds
5. Maintain a list of the best working proxies
6. Log all activities to console and log file

### CLI Management Tool

Use the CLI tool for manual operations:

```bash
# Test proxies from file
./regproxy-cli -action test -count 20

# Crawl new proxies
./regproxy-cli -action crawl

# Validate configuration
./regproxy-cli -action validate

# Show help
./regproxy-cli -help
```

### CLI Options

- **-action**: Action to perform (test, crawl, validate)
- **-config**: Path to config file (default: config.yaml)
- **-file**: Proxy file to use (default: working_proxies.txt)
- **-count**: Number of proxies to test (default: 10)

## Operation Flow

1. **Initial Startup**:
   - Load configuration
   - Try to load existing working proxies
   - If no working proxies, perform initial crawl

2. **Regular Operation**:
   - Test existing working proxies every `interval` seconds
   - Remove non-working proxies
   - Crawl new proxies every `sources_refresh_interval` seconds
   - Keep only the best performing proxies

3. **Proxy Testing**:
   - Each proxy is tested against the ElevenLabs API
   - Tests use actual API calls with your API key
   - Response time and success rate are tracked
   - Only successfully tested proxies are kept

## MongoDB Integration

RegProxy can optionally store proxy data in MongoDB for advanced analytics and persistence.

### MongoDB Features

- **Rich Metadata**: Store proxy IP, port, type, country, anonymity level
- **Performance Tracking**: Record latency, success rate, test count
- **Automatic Indexing**: Optimized queries for performance
- **TTL Collections**: Automatic cleanup of old non-working proxies
- **Statistics**: Advanced analytics and reporting

### MongoDB Document Structure

```json
{
  "_id": "ObjectId",
  "address": "192.168.1.100:8080",
  "ip": "192.168.1.100",
  "port": "8080",
  "type": "http",
  "country": "US",
  "anonymity": "elite",
  "is_working": true,
  "last_tested": "2024-08-03T10:30:00Z",
  "latency_ms": 150,
  "test_count": 45,
  "success_rate": 0.95,
  "created_at": "2024-08-01T09:15:00Z",
  "updated_at": "2024-08-03T10:30:00Z"
}
```

### Setup MongoDB

#### Option 1: Docker Compose (Recommended)

The easiest way to set up MongoDB for development:

```bash
# Start MongoDB with Mongo Express web interface
docker-compose up -d

# View logs
docker-compose logs -f mongodb

# Stop services
docker-compose down
```

This will start:
- MongoDB on port 27017 with authentication
- Mongo Express web interface on http://localhost:8081 (admin/password)

#### Option 2: Manual Installation

1. **Install MongoDB:**
   ```bash
   # macOS with Homebrew
   brew install mongodb-community
   
   # Ubuntu/Debian
   sudo apt-get install mongodb
   ```

2. **Start MongoDB:**
   ```bash
   # macOS with Homebrew
   brew services start mongodb-community
   
   # Ubuntu/Debian
   sudo systemctl start mongodb
   ```

#### Option 3: MongoDB Atlas (Cloud)

Use MongoDB Atlas for production deployments:
1. Create account at https://www.mongodb.com/atlas
2. Create a cluster
3. Get connection string
4. Update DSN in config.yaml

3. **Configure RegProxy:**
   ```yaml
   mongodb:
     enabled: true
     dsn: "mongodb://localhost:27017"
     database: "regproxy"
     collection: "proxy"
     timeout: 10
   ```

### MongoDB Operations

- **Automatic Indexing**: Creates indexes on address, working status, performance metrics
- **Upsert Operations**: Updates existing proxies or creates new ones
- **Success Rate Calculation**: Tracks and calculates success rates over time
- **Data Cleanup**: Removes old non-working proxies automatically (7-day TTL)
- **Query Optimization**: Fast retrieval of working proxies sorted by performance

## Output Files

- **working_proxies.txt**: Current list of working proxies
- **proxies.txt**: All crawled proxies from last crawl
- **daemon.log**: Daemon log file (if configured)

### MongoDB Collections (if enabled)

- **proxy collection**: Rich proxy data with metadata and statistics
- Automatic indexes for optimal query performance
- TTL cleanup for old non-working proxies

- **working_proxies.txt**: Current list of working proxies
- **proxies.txt**: All crawled proxies from last crawl
- **daemon.log**: Daemon log file (if configured)

## Proxy Sources

The application crawls proxies from 100+ sources including:
- GitHub repositories with proxy lists
- Public proxy APIs
- Various proxy list websites

Sources include both HTTP/HTTPS and SOCKS4/SOCKS5 proxies.

## Monitoring

The daemon provides detailed logging:

```
[RegProxy] 2024/08/02 15:30:00 🚀 RegProxy daemon starting...
[RegProxy] 2024/08/02 15:30:00 Loaded 45 working proxies from file
[RegProxy] 2024/08/02 15:30:00 Daemon running with 45 working proxies. Testing every 5m0s
[RegProxy] 2024/08/02 15:35:00 Starting proxy test cycle...
[RegProxy] 2024/08/02 15:35:30 Test completed in 30s. Working proxies: 42/45 (93.33%)
[RegProxy] 2024/08/02 15:35:30 Working proxy: 192.168.1.100:8080
[RegProxy] 2024/08/02 15:35:30 Working proxy: 10.0.0.1:3128
```

## Graceful Shutdown

The daemon handles SIGINT and SIGTERM signals gracefully:
- Saves current working proxies
- Completes ongoing tests
- Closes log files properly

Send CTRL+C or `kill` signal to stop:
```bash
kill -TERM <daemon-pid>
```

## Development

### Project Structure

- **crawler/**: Proxy crawling and basic testing logic
- **api/**: ElevenLabs API specific testing
- **daemon/**: Long-running daemon functionality  
- **config/**: Configuration management
- **cmd/**: Command-line tools

### Adding New Proxy Sources

Edit `crawler/crawler.go` and add new sources to the `getProxySources()` function:

```go
{"https://new-proxy-source.com/proxies.txt", `(\d+\.\d+\.\d+\.\d+):(\d+)`},
```

### Testing

Run tests:
```bash
go test ./...
```

## Troubleshooting

### Common Issues

1. **"please set your ElevenLabs API key"**
   - Edit `config.yaml` and set your actual API key

2. **"No proxies found"**
   - Check internet connection
   - Some sources might be temporarily unavailable
   - Try running with more crawl workers

3. **"All proxies failing tests"**
   - ElevenLabs API might be down
   - Check your API key validity
   - Try reducing concurrent threads

4. **High memory usage**
   - Reduce `test_sample_size` and `keep_working_proxies`
   - Lower `max_crawl_workers`

### Debug Mode

Run with verbose logging:
```bash
./regproxy-daemon -config config.yaml 2>&1 | tee debug.log
```

## License

This project is licensed under the MIT License.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## Support

For issues and questions:
1. Check the troubleshooting section
2. Review the logs in `daemon.log`
3. Open an issue on GitHub
