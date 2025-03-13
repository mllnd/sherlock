package collector

import (
	"time"

	"github.com/mllnd/sherlock/internal/redfish"
	"github.com/mllnd/sherlock/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
)

// SensorCollector collects various sensor metrics
type SensorCollector struct {
	BaseCollector
	temperature       *prometheus.Desc
	voltage           *prometheus.Desc
	temperatureHealth *prometheus.Desc
	voltageHealth     *prometheus.Desc
	readings          map[string]sensorReading
}

type sensorReading struct {
	value      float64
	health     float64
	name       string
	sensorType string
}

// NewSensorCollector creates a new SensorCollector
func NewSensorCollector() *SensorCollector {
	return &SensorCollector{
		BaseCollector: NewBaseCollector("ipmi", "sensor"),
		temperature: prometheus.NewDesc(
			"ipmi_temperature_celsius",
			"Temperature reading in degree Celsius",
			[]string{"name"},
			nil,
		),
		voltage: prometheus.NewDesc(
			"ipmi_voltage_volts",
			"Voltage reading in Volts",
			[]string{"name"},
			nil,
		),
		temperatureHealth: prometheus.NewDesc(
			"ipmi_temperature_health",
			"Temperature sensor health status (1 = OK, 0 = Warning/Critical, 2 = Not Available)",
			[]string{"name"},
			nil,
		),
		voltageHealth: prometheus.NewDesc(
			"ipmi_voltage_health",
			"Voltage sensor health status (1 = OK, 0 = Warning/Critical, 2 = Not Available)",
			[]string{"name"},
			nil,
		),
		readings: make(map[string]sensorReading),
	}
}

// Update fetches new metrics and updates the prometheus metrics
func (c *SensorCollector) Update(client *redfish.Client) error {
	start := time.Now()
	defer c.RecordScrapeTime(start)

	// Clear previous readings
	c.mutex.Lock()
	c.readings = make(map[string]sensorReading)
	c.mutex.Unlock()

	// Get Chassis ID 1
	chassis, err := client.GetChassisWithID("1")
	if err != nil {
		c.logger.Debug("failed to get main chassis", "error", err)
		return nil
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Get and process temperature sensors
	thermal, err := chassis.Thermal()
	if err != nil {
		c.logger.Debug("failed to get thermal information", "error", err)
		return nil
	}

	// Process temperature sensors
	for _, temp := range thermal.Temperatures {
		if temp.Name == "" {
			continue
		}

		health := 2.0 // Default to Not Available
		if temp.Status.Health != "" {
			if temp.Status.Health == "OK" {
				health = 1.0
			} else {
				health = 0.0
			}
		}

		c.readings[temp.Name] = sensorReading{
			value:      float64(temp.ReadingCelsius),
			health:     health,
			name:       temp.Name,
			sensorType: "temperature",
		}
	}

	// Get and process voltage sensors
	power, err := chassis.Power()
	if err != nil {
		// If we can't get power info, but we have temperature readings,
		// return success as we at least have some data
		if len(c.readings) > 0 {
			return nil
		}
		c.logger.Debug("failed to get power information", "error", err)
		return nil
	}

	// Process all voltage sensors
	for _, volt := range power.Voltages {
		if volt.Name == "" {
			continue
		}

		health := 2.0 // Default to Not Available
		if volt.Status.Health != "" {
			if volt.Status.Health == "OK" {
				health = 1.0
			} else {
				health = 0.0
			}
		}

		c.readings[volt.Name] = sensorReading{
			value:      utils.Round(float64(volt.ReadingVolts), 3),
			health:     health,
			name:       volt.Name,
			sensorType: "voltage",
		}
	}

	return nil
}

// Describe describes all metrics this collector exposes
func (c *SensorCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.temperature
	ch <- c.voltage
	ch <- c.temperatureHealth
	ch <- c.voltageHealth
	c.DescribeScrapeTime(ch)
}

// Collect collects all metrics
func (c *SensorCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, reading := range c.readings {
		switch reading.sensorType {
		case "temperature":
			ch <- prometheus.MustNewConstMetric(
				c.temperature,
				prometheus.GaugeValue,
				reading.value,
				reading.name,
			)
			ch <- prometheus.MustNewConstMetric(
				c.temperatureHealth,
				prometheus.GaugeValue,
				reading.health,
				reading.name,
			)
		case "voltage":
			ch <- prometheus.MustNewConstMetric(
				c.voltage,
				prometheus.GaugeValue,
				reading.value,
				reading.name,
			)
			ch <- prometheus.MustNewConstMetric(
				c.voltageHealth,
				prometheus.GaugeValue,
				reading.health,
				reading.name,
			)
		}
	}

	c.CollectScrapeTime(ch)
}
