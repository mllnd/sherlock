package collector

import (
	"time"

	"github.com/mllnd/sherlock/internal/redfish"
	"github.com/prometheus/client_golang/prometheus"
)

// FansCollector collects fan metrics
type FansCollector struct {
	BaseCollector
	health *prometheus.Desc
	state  *prometheus.Desc
	speed  *prometheus.Desc
	fans   map[string]fanMetric
}

type fanMetric struct {
	health float64
	state  float64
	speed  float64
	name   string
}

// NewFansCollector creates a new FansCollector
func NewFansCollector() *FansCollector {
	return &FansCollector{
		BaseCollector: NewBaseCollector("ipmi", "fan"),
		health: prometheus.NewDesc(
			"ipmi_fan_health",
			"Fan health status (1 = OK, 0 = Warning/Critical, 2 = Not Available)",
			[]string{"name"},
			nil,
		),
		state: prometheus.NewDesc(
			"ipmi_fan_state",
			"Fan operating state (1 = Enabled, 0 = Disabled)",
			[]string{"name"},
			nil,
		),
		speed: prometheus.NewDesc(
			"ipmi_fan_speed_rpm",
			"Fan speed in RPM",
			[]string{"name"},
			nil,
		),
		fans: make(map[string]fanMetric),
	}
}

// Update fetches new metrics and updates the prometheus metrics
func (c *FansCollector) Update(client *redfish.Client) error {
	start := time.Now()
	defer c.RecordScrapeTime(start)

	// Clear previous readings
	c.mutex.Lock()
	c.fans = make(map[string]fanMetric)
	c.mutex.Unlock()

	// Get main chassis (ID 1)
	chassis, err := client.GetMainChassis()
	if err != nil {
		c.logger.Debug("failed to get main chassis", "error", err)
		return nil
	}

	// Get thermal information
	thermal, err := chassis.Thermal()
	if err != nil {
		c.logger.Debug("failed to get thermal information", "error", err)
		return nil
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Process all fans
	for _, fan := range thermal.Fans {
		// Skip if no readings available
		if fan.Name == "" {
			continue
		}

		// Convert health status to float64
		health := 2.0 // Default to Not Available
		if fan.Status.Health != "" {
			if fan.Status.Health == "OK" {
				health = 1.0
			} else {
				health = 0.0
			}
		}

		// Convert operating state to float64
		state := 0.0
		if fan.Status.State == "Enabled" {
			state = 1.0
		}

		c.fans[fan.Name] = fanMetric{
			health: health,
			state:  state,
			speed:  float64(fan.Reading),
			name:   fan.Name,
		}
	}

	return nil
}

// Describe describes all metrics this collector exposes
func (c *FansCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.health
	ch <- c.state
	ch <- c.speed
	c.DescribeScrapeTime(ch)
}

// Collect collects all metrics
func (c *FansCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, reading := range c.fans {
		ch <- prometheus.MustNewConstMetric(
			c.health,
			prometheus.GaugeValue,
			reading.health,
			reading.name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.state,
			prometheus.GaugeValue,
			reading.state,
			reading.name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.speed,
			prometheus.GaugeValue,
			reading.speed,
			reading.name,
		)
	}

	c.CollectScrapeTime(ch)
}
