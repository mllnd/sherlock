package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/mllnd/sherlock/internal/collector"
	"github.com/mllnd/sherlock/internal/config"
	"github.com/mllnd/sherlock/internal/logging"
	"github.com/mllnd/sherlock/internal/redfish"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Build information, set via -ldflags
var (
	version = "dev"
	commit  = "none"
)

var (
	listenAddress = flag.String("web.listen-address", "localhost:9290", "Address to listen on for web interface and telemetry")
	metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics")
	showVersion   = flag.Bool("version", false, "Print version information and exit")
)

// SherlockCollector is the main collector that wraps all other collectors
type SherlockCollector struct {
	config  *config.Config
	clients map[string]*redfish.Client
	mutex   sync.Mutex
	logger  *logging.Logger
}

// NewSherlockCollector creates a new SherlockCollector
func NewSherlockCollector(config *config.Config) (*SherlockCollector, error) {
	return &SherlockCollector{
		config:  config,
		clients: make(map[string]*redfish.Client),
		logger:  logging.New(),
	}, nil
}

// getClient returns a Redfish client for the given target hostname
func (c *SherlockCollector) getClient(hostname string) (*redfish.Client, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// If we already have a client for this target, return it
	if client, ok := c.clients[hostname]; ok {
		return client, nil
	}

	// Create a new client for this target
	targetURL := "https://" + hostname
	redfishConfig := redfish.Config{
		Host:     targetURL,
		Username: c.config.RedfishUsername,
		Password: c.config.RedfishPassword,
		Insecure: c.config.RedfishInsecure,
	}

	client, err := redfish.NewClient(redfishConfig)
	if err != nil {
		return nil, err
	}

	// Store the client for future use
	c.clients[hostname] = client
	return client, nil
}

// Describe implements the prometheus.Collector interface
func (c *SherlockCollector) Describe(ch chan<- *prometheus.Desc) {
	// Create temporary collectors to describe metrics
	collectors := []collector.Collector{
		collector.NewSystemCollector(),
		collector.NewSensorCollector(),
		collector.NewPowerCollector(),
		collector.NewFansCollector(),
		collector.NewTelemetryCollector(),
	}

	for _, collector := range collectors {
		collector.Describe(ch)
	}
}

// Collect implements the prometheus.Collector interface
func (c *SherlockCollector) Collect(ch chan<- prometheus.Metric) {
	// This method is required to implement the prometheus.Collector interface
	// but it's not used since we're using the targetCollector wrapper
	c.logger.Warn("Collect method called without a target")
}

// collectTarget collects metrics for a specific target
func (c *SherlockCollector) collectTarget(ch chan<- prometheus.Metric, target string) {
	// Get or create a client for this target
	client, err := c.getClient(target)
	if err != nil {
		c.logger.Error("failed to connect to redfish api", "target", target, "error", err)
		return
	}

	// Create new collectors for this target
	collectors := []collector.Collector{
		collector.NewSystemCollector(),
		collector.NewSensorCollector(),
		collector.NewPowerCollector(),
		collector.NewFansCollector(),
		collector.NewTelemetryCollector(),
	}

	// Set target on each collector
	for _, col := range collectors {
		col.SetTarget(target)
	}

	// Create a wait group to wait for all collectors to finish
	var wg sync.WaitGroup
	wg.Add(len(collectors))

	// Create a channel to collect errors
	errChan := make(chan error, len(collectors))

	// Update all collectors in parallel
	for i := range collectors {
		go func(index int) {
			defer wg.Done()
			if err := collectors[index].Update(client); err != nil {
				errChan <- fmt.Errorf("error updating collector %T for target %s: %v", collectors[index], target, err)
			}
		}(i)
	}

	// Wait for all collectors to finish
	wg.Wait()
	close(errChan)

	// Log any errors
	for err := range errChan {
		c.logger.Error("collector update failed", "error", err)
	}

	// Collect metrics from all collectors
	for _, collector := range collectors {
		collector.Collect(ch)
	}
}

// Close closes all Redfish clients
func (c *SherlockCollector) Close() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for target, client := range c.clients {
		client.Close()
		delete(c.clients, target)
	}
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("sherlock %s-%s\n", version, commit)
		os.Exit(0)
	}

	logger := logging.New()

	// Load configuration
	cfg := config.NewConfig()

	// Create collector
	collector, err := NewSherlockCollector(cfg)
	if err != nil {
		logger.Error("Failed to create collector", "error", err)
		os.Exit(1)
	}
	defer collector.Close()

	// Create a custom handler for metrics that supports the target parameter
	http.HandleFunc(*metricsPath, func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")

		if target == "" {
			http.Error(w, "Error: 'target' parameter is required (e.g. ?target=bmc.example.com)", http.StatusBadRequest)
			return
		}

		// Remove any protocol prefix if accidentally included
		target = strings.TrimPrefix(target, "http://")
		target = strings.TrimPrefix(target, "https://")

		logger.Debug("starting metrics collection",
			"target", target,
			"goroutine", fmt.Sprintf("%p", &target),
		)

		registry := prometheus.NewRegistry()
		registry.MustRegister(&targetCollector{collector: collector, target: target})

		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)

		logger.Debug("finished metrics collection",
			"target", target,
			"goroutine", fmt.Sprintf("%p", &target),
		)
	})

	// Create index page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Sherlock Redfish Exporter</title></head>
			<body>
			<h1>Sherlock Redfish Exporter</h1>
			<p>This exporter requires a target parameter (hostname only):</p>
			<p><a href="` + *metricsPath + `?target=bmc.example.com">` + *metricsPath + `?target=bmc.example.com</a></p>
			</body>
			</html>`))
	})

	// Start HTTP server
	logger.Info("starting sherlock redfish exporter", "address", *listenAddress)

	// Handle graceful shutdown
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		<-signals
		logger.Info("shutting down...")
		collector.Close()
		os.Exit(0)
	}()

	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		logger.Error("http server failed", "error", err)
		os.Exit(1)
	}
}

// targetCollector is a wrapper around SherlockCollector that collects metrics for a specific target
type targetCollector struct {
	collector *SherlockCollector
	target    string
}

func (tc *targetCollector) Describe(ch chan<- *prometheus.Desc) {
	tc.collector.Describe(ch)
}

func (tc *targetCollector) Collect(ch chan<- prometheus.Metric) {
	tc.collector.collectTarget(ch, tc.target)
}
