package collector

import (
	"strings"
	"sync"
	"time"

	"github.com/mllnd/sherlock/internal/logging"
	"github.com/mllnd/sherlock/internal/redfish"
	"github.com/prometheus/client_golang/prometheus"
)

// Collector is the interface that all collectors must implement
type Collector interface {
	// Update fetches new metrics and updates the prometheus metrics
	Update(client *redfish.Client) error

	// Describe describes all metrics this collector exposes
	Describe(ch chan<- *prometheus.Desc)

	// Collect collects all metrics
	Collect(ch chan<- prometheus.Metric)

	// SetTarget sets the target being scraped
	SetTarget(target string)
}

// BaseCollector provides common functionality for all collectors
type BaseCollector struct {
	mutex       sync.Mutex
	lastCollect time.Time
	scrapeTime  prometheus.Gauge
	logger      *logging.Logger
	target      string
}

// NewBaseCollector creates a new BaseCollector
func NewBaseCollector(namespace, subsystem string) BaseCollector {
	return BaseCollector{
		scrapeTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "scrape_duration_seconds",
			Help:      "Duration of the last scrape in seconds",
		}),
		logger: logging.New(),
	}
}

// SetTarget sets the target being scraped
func (c *BaseCollector) SetTarget(target string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.target = target
}

// RecordScrapeTime records the time taken to scrape metrics
func (c *BaseCollector) RecordScrapeTime(start time.Time) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	duration := time.Since(start).Seconds()
	c.lastCollect = time.Now()
	c.scrapeTime.Set(duration)

	// Extract subsystem name from the metric description
	desc := c.scrapeTime.Desc().String()
	subsystem := "unknown"
	if start := strings.Index(desc, "ipmi_"); start != -1 {
		if end := strings.Index(desc[start:], "_scrape"); end != -1 {
			subsystem = desc[start : start+end]
		}
	}

	c.logger.Debugw("scrape completed",
		"duration_seconds", duration,
		"subsystem", subsystem,
		"target", c.target,
	)
}

// DescribeScrapeTime describes the scrape time metric
func (c *BaseCollector) DescribeScrapeTime(ch chan<- *prometheus.Desc) {
	c.scrapeTime.Describe(ch)
}

// CollectScrapeTime collects the scrape time metric
func (c *BaseCollector) CollectScrapeTime(ch chan<- prometheus.Metric) {
	c.scrapeTime.Collect(ch)
}
