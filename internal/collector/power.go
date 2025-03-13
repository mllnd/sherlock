package collector

import (
	"fmt"
	"time"

	"github.com/mllnd/sherlock/internal/redfish"
	"github.com/prometheus/client_golang/prometheus"
)

// PowerCollector collects power supply metrics
type PowerCollector struct {
	BaseCollector
	psuHealth       *prometheus.Desc
	psuACInputPower *prometheus.Desc
	psuDCPower      *prometheus.Desc
	readings        map[string]psuReading
}

type psuReading struct {
	health  float64
	acPower float64
	dcPower float64
	name    string
}

// NewPowerCollector creates a new PowerCollector
func NewPowerCollector() *PowerCollector {
	return &PowerCollector{
		BaseCollector: NewBaseCollector("ipmi", "power"),
		psuHealth: prometheus.NewDesc(
			"ipmi_psu_health",
			"Power supply health status (1 = OK, 0 = Warning/Critical, 2 = Not Available)",
			[]string{"name"},
			nil,
		),
		psuACInputPower: prometheus.NewDesc(
			"ipmi_psu_ac_input_power_watts",
			"Power supply AC input power in watts",
			[]string{"name"},
			nil,
		),
		psuDCPower: prometheus.NewDesc(
			"ipmi_psu_dc_output_power_watts",
			"Power supply DC output power in watts",
			[]string{"name"},
			nil,
		),
		readings: make(map[string]psuReading),
	}
}

// Update fetches new metrics and updates the prometheus metrics
func (c *PowerCollector) Update(client *redfish.Client) error {
	start := time.Now()
	defer c.RecordScrapeTime(start)

	// Clear previous readings first to ensure we don't have stale data
	c.mutex.Lock()
	c.readings = make(map[string]psuReading)
	c.mutex.Unlock()

	// Try to get the primary chassis
	chassis, err := client.GetChassisWithID("1")
	if err != nil {
		c.logger.Debug("failed to get primary chassis", "error", err)
		return c.tryAlternativeChassis(client)
	}

	// Get power information
	power, err := chassis.Power()
	if err != nil {
		c.logger.Debug("failed to get power information from primary chassis", "error", err)
		return c.tryAlternativeChassis(client)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	psuCount := 0
	// Process power supplies
	for _, psu := range power.PowerSupplies {
		// Skip if no readings available
		if psu.Name == "" {
			continue
		}
		psuCount++

		// Convert health status to float64
		health := 2.0 // Default to Not Available
		if psu.Status.Health != "" {
			if psu.Status.Health == "OK" {
				health = 1.0
			} else {
				health = 0.0
			}
		}

		c.readings[psu.Name] = psuReading{
			health:  health,
			name:    fmt.Sprintf("PSU %d", psuCount),
			acPower: float64(psu.PowerInputWatts),
			dcPower: float64(psu.PowerOutputWatts),
		}
	}

	return nil
}

// tryAlternativeChassis attempts to find any working chassis to get power information
func (c *PowerCollector) tryAlternativeChassis(client *redfish.Client) error {
	// Get all chassis
	allChassis, err := client.GetChassis()
	if err != nil {
		return fmt.Errorf("failed to get any chassis: %v", err)
	}

	// Try each chassis until we find one that works
	for _, chassis := range allChassis {
		// Skip the problematic storage backplane
		if chassis.ID == "NVMeSSD.0.Group.0.StorageBackplane" {
			c.logger.Debug("skipping known problematic chassis", "id", chassis.ID)
			continue
		}

		// Try to get power information from this chassis
		power, err := chassis.Power()
		if err != nil {
			c.logger.Debug("failed to get power information", "chassis", chassis.ID, "error", err)
			continue
		}

		c.mutex.Lock()
		defer c.mutex.Unlock()

		psuCount := 0
		// Process power supplies
		for _, psu := range power.PowerSupplies {
			// Skip if no readings available
			if psu.Name == "" {
				continue
			}
			psuCount++

			// Convert health status to float64
			health := 2.0 // Default to Not Available
			if psu.Status.Health != "" {
				if psu.Status.Health == "OK" {
					health = 1.0
				} else {
					health = 0.0
				}
			}

			c.readings[psu.Name] = psuReading{
				health:  health,
				name:    fmt.Sprintf("PSU %d", psuCount),
				acPower: float64(psu.PowerInputWatts),
				dcPower: float64(psu.PowerOutputWatts),
			}
		}

		// If we found power supplies, we're done
		if psuCount > 0 {
			c.logger.Debug("successfully retrieved power information", "chassis", chassis.ID)
			return nil
		}
	}

	// If we get here, we couldn't find any working chassis with power supplies
	if len(c.readings) == 0 {
		return fmt.Errorf("could not find any chassis with power supply information")
	}

	return nil
}

// Describe describes all metrics this collector exposes
func (c *PowerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.psuHealth
	ch <- c.psuACInputPower
	ch <- c.psuDCPower
	c.DescribeScrapeTime(ch)
}

// Collect collects all metrics
func (c *PowerCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, reading := range c.readings {
		ch <- prometheus.MustNewConstMetric(
			c.psuHealth,
			prometheus.GaugeValue,
			reading.health,
			reading.name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.psuACInputPower,
			prometheus.GaugeValue,
			reading.acPower,
			reading.name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.psuDCPower,
			prometheus.GaugeValue,
			reading.dcPower,
			reading.name,
		)
	}

	c.CollectScrapeTime(ch)
}
