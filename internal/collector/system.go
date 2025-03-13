package collector

import (
	"fmt"
	"time"

	"github.com/mllnd/sherlock/internal/redfish"
	"github.com/prometheus/client_golang/prometheus"
)

// SystemCollector collects system-level metrics
type SystemCollector struct {
	BaseCollector
	powerState   *prometheus.Desc
	cpuHealth    *prometheus.Desc
	memoryHealth *prometheus.Desc
	readings     map[string]systemReading
	firstCPUID   string
}

type systemReading struct {
	powerState     float64
	health         float64
	cores          float64
	name           string
	model          string
	totalMemoryGiB string
}

// NewSystemCollector creates a new SystemCollector
func NewSystemCollector() *SystemCollector {
	return &SystemCollector{
		BaseCollector: NewBaseCollector("ipmi", "system"),
		powerState: prometheus.NewDesc(
			"ipmi_system_power_state",
			"System power state (1 = On, 0 = Off)",
			nil,
			nil,
		),
		cpuHealth: prometheus.NewDesc(
			"ipmi_cpu_health",
			"CPU health status (1 = OK, 0 = Warning/Critical, 2 = Not Available)",
			[]string{"name", "model", "cores"},
			nil,
		),
		memoryHealth: prometheus.NewDesc(
			"ipmi_memory_health",
			"Overall memory subsystem health status (1 = OK, 0 = Warning/Critical, 2 = Not Available)",
			[]string{"total_gib"},
			nil,
		),
		readings: make(map[string]systemReading),
	}
}

// Update fetches new metrics and updates the prometheus metrics
func (c *SystemCollector) Update(client *redfish.Client) error {
	start := time.Now()
	defer c.RecordScrapeTime(start)

	// Get all systems
	systems, err := client.Service.Systems()
	if err != nil {
		c.logger.Debug("failed to get systems", "error", err)
		return nil
	}

	// Get the first system
	system := systems[0]

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Clear previous readings
	c.readings = make(map[string]systemReading)

	// Get power state
	powerState := 0.0
	if system.PowerState == "On" {
		powerState = 1.0
	}

	// Get CPU information
	processors, err := system.Processors()
	if err != nil {
		c.logger.Debug("failed to get processors", "error", err)
		return nil
	}

	// Store first CPU ID for power state reporting
	if len(processors) > 0 {
		c.firstCPUID = processors[0].ID
	}

	// Process each CPU
	for _, cpu := range processors {
		health := 2.0 // Default to Not Available
		if cpu.Status.Health != "" {
			if cpu.Status.Health == "OK" {
				health = 1.0
			} else {
				health = 0.0
			}
		}

		c.readings[cpu.ID] = systemReading{
			powerState: powerState,
			health:     health,
			cores:      float64(cpu.TotalCores),
			name:       cpu.ID,
			model:      cpu.Model,
		}
	}

	// Get memory health from system status
	memoryHealth := 2.0 // Default to Not Available
	totalMemoryGiB := fmt.Sprintf("%.0f", float64(system.MemorySummary.TotalSystemMemoryGiB))
	if system.MemorySummary.Status.Health != "" {
		if system.MemorySummary.Status.Health == "OK" {
			memoryHealth = 1.0
		} else {
			memoryHealth = 0.0
		}
	}

	// Store memory health in the first CPU reading
	if len(c.readings) > 0 {
		reading := c.readings[c.firstCPUID]
		reading.health = memoryHealth
		reading.powerState = powerState
		reading.totalMemoryGiB = totalMemoryGiB
		c.readings[c.firstCPUID] = reading
	}

	return nil
}

// Describe describes all metrics this collector exposes
func (c *SystemCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.powerState
	ch <- c.cpuHealth
	ch <- c.memoryHealth
	c.DescribeScrapeTime(ch)
}

// Collect collects all metrics
func (c *SystemCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, reading := range c.readings {
		// Report power state and memory health only once since they're system-wide
		if reading.name == c.firstCPUID {
			ch <- prometheus.MustNewConstMetric(
				c.powerState,
				prometheus.GaugeValue,
				reading.powerState,
			)

			ch <- prometheus.MustNewConstMetric(
				c.memoryHealth,
				prometheus.GaugeValue,
				reading.health,
				reading.totalMemoryGiB,
			)
		}

		ch <- prometheus.MustNewConstMetric(
			c.cpuHealth,
			prometheus.GaugeValue,
			reading.health,
			reading.name,
			reading.model,
			fmt.Sprintf("%d", int(reading.cores)),
		)
	}

	c.CollectScrapeTime(ch)
}
