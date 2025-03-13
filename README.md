# sherlock

Sherlock is a Prometheus exporter that collects metrics from Redfish-enabled BMCs, focusing on system, temperature, voltage, power, and fan metrics.

## Features

- Collects comprehensive metrics from Redfish-enabled servers
- Supports monitoring multiple servers with a single exporter instance
- Parallel collection of metrics for improved performance
- Detailed health status for most server components

## Building

### Local Build
```bash
go build -o sherlock ./cmd/sherlock
```

### Docker Build
```bash
docker build -t sherlock .
```

## Running

### Local Run
```bash
export REDFISH_USERNAME="admin"
export REDFISH_PASSWORD="password"
./sherlock
```

### Docker Compose
```bash
# Set your BMC credentials in environment
export REDFISH_USERNAME="admin"
export REDFISH_PASSWORD="password"

# Run with docker-compose
docker-compose up -d
```

## Configuration

The following environment variables are available:

- `REDFISH_USERNAME`: BMC username (default: "admin")
- `REDFISH_PASSWORD`: BMC password (default: "password")
- `REDFISH_INSECURE`: Allow insecure HTTPS connections (default: true)
- `LISTEN_ADDRESS`: Address to listen on (default: ":9290")
- `METRICS_PATH`: Path to expose metrics on (default: "/metrics")

## Multi-Server Monitoring

Sherlock requires a target parameter to specify which server to monitor. The target parameter should be just the hostname of the Redfish endpoint (HTTPS is used automatically):

```
http://sherlock:9290/metrics?target=bmc1.example.com
http://sherlock:9290/metrics?target=bmc2.example.com
```

If no target is specified, the metrics endpoint will return an error.

In your Prometheus configuration, you can use this feature with relabeling:

```yaml
scrape_configs:
  - job_name: "redfish"
    scrape_interval: "60s"
    metrics_path: "/metrics"
    params:
      target: ["bmc1.example.com", "bmc2.example.com"]
    static_configs:
      - targets: ["sherlock:9290"]
    relabel_configs:
      - source_labels: [__param_target]
        target_label: "instance"
```

This allows you to use a single Sherlock instance to monitor multiple servers while keeping the same credentials.

## Metrics

The exporter provides the following metrics:

### System Metrics
- `ipmi_system_power_state`: System power state (1 = On, 0 = Off)
- `ipmi_cpu_health`: CPU health status with model and core count as labels
- `ipmi_memory_health`: Overall memory subsystem health status with total memory size

### Temperature Metrics
- `ipmi_temperature_celsius`: Temperature readings in Celsius with type labels
- `ipmi_temperature_health`: Health status of temperature sensors

### Voltage Metrics
- `ipmi_voltage_volts`: Voltage readings in Volts
- `ipmi_voltage_health`: Health status of voltage sensors

### Power Supply Metrics
- `ipmi_psu_health`: Power supply health status
- `ipmi_psu_input_power_watts`: Power supply AC input power in Watts
- `ipmi_psu_output_power_watts`: Power supply DC output power in Watts

### Fan Metrics
- `ipmi_fan_health`: Fan health status
- `ipmi_fan_state`: Fan state (1 = On, 0 = Off)
- `ipmi_fan_speed_rpm`: Fan speed in RPM
