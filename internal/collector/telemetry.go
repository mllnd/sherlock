package collector

import (
	"time"

	"github.com/mllnd/sherlock/internal/redfish"
	"github.com/prometheus/client_golang/prometheus"
)

// TelemetryCollector collects power consumption metrics
type TelemetryCollector struct {
	BaseCollector
	powerConsumption *prometheus.Desc
	reading          float64
}

// NewTelemetryCollector creates a new TelemetryCollector
func NewTelemetryCollector() *TelemetryCollector {
	return &TelemetryCollector{
		BaseCollector: NewBaseCollector("ipmi", "telemetry"),
		powerConsumption: prometheus.NewDesc(
			"ipmi_telemetry_power_consumption_watts",
			"Current power consumption in watts",
			nil,
			nil,
		),
	}
}

// Update fetches new metrics and updates the prometheus metrics
func (c *TelemetryCollector) Update(client *redfish.Client) error {
	start := time.Now()
	defer c.RecordScrapeTime(start)

	// Clear previous reading
	c.mutex.Lock()
	c.reading = 0
	c.mutex.Unlock()

	// Try to get power consumption from chassis
	chassis, err := client.GetMainChassis()
	if err != nil {
		c.logger.Debug("failed to get main chassis", "error", err)
		return nil
	}

	power, err := chassis.Power()
	if err != nil {
		c.logger.Debug("failed to get power information", "error", err)
		return nil
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Process power control readings
	for _, pc := range power.PowerControl {
		if pc.PowerConsumedWatts > 0 {
			c.reading = float64(pc.PowerConsumedWatts)
			c.logger.Debug("updated power consumption", "watts", c.reading)
			break // Take the first valid reading
		}
	}

	return nil
}

// Describe describes all metrics this collector exposes
func (c *TelemetryCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.powerConsumption
	c.DescribeScrapeTime(ch)
}

// Collect collects all metrics
func (c *TelemetryCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.reading > 0 {
		ch <- prometheus.MustNewConstMetric(
			c.powerConsumption,
			prometheus.GaugeValue,
			c.reading,
		)
	}

	c.CollectScrapeTime(ch)
}
