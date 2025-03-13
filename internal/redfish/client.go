package redfish

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
)

// Client wraps the gofish API client with additional functionality
type Client struct {
	*gofish.APIClient
	Service *gofish.Service
	config  Config
	mutex   sync.Mutex
}

// Config holds the configuration for the Redfish client
type Config struct {
	Host     string
	Username string
	Password string
	Insecure bool
}

// NewConfig creates a new Config with values from environment or defaults
func NewConfig() Config {
	return Config{
		Host:     getEnv("REDFISH_HOST", "http://localhost:5000"),
		Username: getEnv("REDFISH_USERNAME", "admin"),
		Password: getEnv("REDFISH_PASSWORD", "password"),
		Insecure: true,
	}
}

// NewClient creates a new Redfish client
func NewClient(config Config) (*Client, error) {
	client, err := connect(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// connect establishes a new connection to the Redfish API
func connect(config Config) (*Client, error) {
	goConfig := gofish.ClientConfig{
		Endpoint: config.Host,
		Username: config.Username,
		Password: config.Password,
		Insecure: config.Insecure,
	}

	apiClient, err := gofish.Connect(goConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redfish API: %v", err)
	}

	client := &Client{
		APIClient: apiClient,
		Service:   apiClient.Service,
		config:    config,
	}

	return client, nil
}

// reconnect attempts to establish a new connection
func (c *Client) reconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Close existing connection if any
	if c.APIClient != nil {
		c.Close()
	}

	// Create new connection
	newClient, err := connect(c.config)
	if err != nil {
		return err
	}

	// Update client with new connection
	c.APIClient = newClient.APIClient
	c.Service = newClient.Service

	return nil
}

// GetChassis returns all chassis from the Redfish API
func (c *Client) GetChassis() ([]*redfish.Chassis, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.Service.Chassis()
}

// GetMainChassis returns the main chassis (ID "1") from the Redfish API
func (c *Client) GetMainChassis() (*redfish.Chassis, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	chassis, err := c.Service.Chassis()
	if err != nil {
		// Check if we got a partial response
		if strings.Contains(err.Error(), "failed to retrieve some items") && len(chassis) > 0 {
			// Continue with the chassis we got
		} else if isAuthError(err) {
			// Try to reconnect and retry once
			if reconnectErr := c.reconnect(); reconnectErr != nil {
				return nil, fmt.Errorf("failed to reconnect: %v (original error: %v)", reconnectErr, err)
			}
			chassis, err = c.Service.Chassis()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Safety check
	if len(chassis) == 0 {
		return nil, fmt.Errorf("no chassis found")
	}

	// Look for chassis ID 1
	for _, ch := range chassis {
		if ch != nil && ch.ID == "1" {
			return ch, nil
		}
	}
	return nil, fmt.Errorf("main chassis (ID 1) not found")
}

// filterNumericChassis returns only chassis with numeric IDs
func filterNumericChassis(chassis []*redfish.Chassis) []*redfish.Chassis {
	var result []*redfish.Chassis
	for _, ch := range chassis {
		// Keep only chassis with numeric IDs (0, 1, 2, etc.)
		if len(ch.ID) == 1 && ch.ID[0] >= '0' && ch.ID[0] <= '9' {
			result = append(result, ch)
		}
	}
	return result
}

// GetChassisWithID returns a specific chassis by ID with automatic session renewal
func (c *Client) GetChassisWithID(id string) (*redfish.Chassis, error) {
	if id == "1" {
		return c.GetMainChassis()
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	chassis, err := c.Service.Chassis()
	if err != nil {
		// Check if we got a partial response
		if strings.Contains(err.Error(), "failed to retrieve some items") && len(chassis) > 0 {
			// Continue with the chassis we got
		} else if isAuthError(err) {
			// Try to reconnect and retry once
			if reconnectErr := c.reconnect(); reconnectErr != nil {
				return nil, fmt.Errorf("failed to reconnect: %v (original error: %v)", reconnectErr, err)
			}
			chassis, err = c.Service.Chassis()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Safety check
	if len(chassis) == 0 {
		return nil, fmt.Errorf("no chassis found")
	}

	// Look for the requested chassis
	for _, ch := range chassis {
		if ch != nil && ch.ID == id {
			return ch, nil
		}
	}
	return nil, fmt.Errorf("chassis with ID %s not found", id)
}

// isAuthError checks if the error is an authentication error
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "session expired")
}

// Close properly closes the Redfish connection
func (c *Client) Close() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.APIClient != nil {
		c.Logout()
	}
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
