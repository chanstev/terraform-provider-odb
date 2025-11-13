package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config files are loaded from disk at runtime
// Embedding is complex with relative paths, so we'll use direct file access

// FieldMapping represents a field mapping configuration
type FieldMapping struct {
	TerraformField string
	CloudMappings  map[string]CloudFieldMapping // key: "azure", "oci"
}

// CloudFieldMapping represents how a field maps to a cloud API
type CloudFieldMapping struct {
	APIPath    string // e.g., "azure.properties.displayName" or "oci.displayName"
	Operations string // e.g., "c*r*u" - operations where this mapping applies
	IsRequired bool   // true if contains "*"
	IsMaster   bool   // true if contains "r*"
	Regex      string // optional regex for extraction
}

// ResourceConfig represents the YAML configuration for a resource
type ResourceConfig struct {
	Azure *CloudResourceConfig `yaml:"azure"`
	OCI   *CloudResourceConfig `yaml:"oci"`
}

// CloudResourceConfig represents cloud-specific resource configuration
type CloudResourceConfig struct {
	FieldMapping map[string]interface{} `yaml:"field_mapping"`
	Create       string                 `yaml:"create"`
	Read         string                 `yaml:"read"`
	Update       string                 `yaml:"update"`
	Delete       string                 `yaml:"delete"`
	OCIRead      string                 `yaml:"oci_read"`
	OCIUpdate    string                 `yaml:"oci_update"`
}

// GlobalConfig represents the global.yaml configuration
type GlobalConfig struct {
	DefaultTimeout int `yaml:"default_timeout"`
	Retry          struct {
		MaxAttempts       int     `yaml:"max_attempts"`
		InitialDelayMs    int     `yaml:"initial_delay_ms"`
		MaxDelayMs        int     `yaml:"max_delay_ms"`
		BackoffMultiplier float64 `yaml:"backoff_multiplier"`
	} `yaml:"retry"`
	AsyncOperations struct {
		Azure struct {
			StatusField       string `yaml:"status_field"`
			ProvisioningField string `yaml:"provisioning_field"`
			TerminalStates    struct {
				Lifecycle    []string `yaml:"lifecycle"`
				Provisioning []string `yaml:"provisioning"`
			} `yaml:"terminal_states"`
			InProgressStates struct {
				Lifecycle    []string `yaml:"lifecycle"`
				Provisioning []string `yaml:"provisioning"`
			} `yaml:"in_progress_states"`
			PollIntervalSeconds             int  `yaml:"poll_interval_seconds"`
			MaxPollDurationMinutes          int  `yaml:"max_poll_duration_minutes"`
			WaitForAvailableBeforeOCIUpdate bool `yaml:"wait_for_available_before_oci_update"`
		} `yaml:"azure"`
		OCI struct {
			StatusField            string   `yaml:"status_field"`
			TerminalStates         []string `yaml:"terminal_states"`
			InProgressStates       []string `yaml:"in_progress_states"`
			PollIntervalSeconds    int      `yaml:"poll_interval_seconds"`
			MaxPollDurationMinutes int      `yaml:"max_poll_duration_minutes"`
		} `yaml:"oci"`
	} `yaml:"async_operations"`
}

// ConfigLoader loads and parses YAML configurations
type ConfigLoader struct {
	ConfigPath      string
	GlobalConfig    *GlobalConfig
	ResourceConfigs map[string]*ResourceConfig
}

// NewConfigLoader creates a new config loader with default path and loads all configs
func NewConfigLoader() (*ConfigLoader, error) {
	return NewConfigLoaderWithPath("config")
}

// NewConfigLoaderWithPath creates a new config loader with custom path and loads all configs
func NewConfigLoaderWithPath(configPath string) (*ConfigLoader, error) {
	loader := &ConfigLoader{
		ConfigPath:      configPath,
		ResourceConfigs: make(map[string]*ResourceConfig),
	}

	// Load global config
	if err := loader.loadGlobalConfig(); err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	// Load resource configs
	if err := loader.loadResourceConfig("autonomous_database"); err != nil {
		return nil, fmt.Errorf("failed to load autonomous_database config: %w", err)
	}

	return loader, nil
}

// loadGlobalConfig loads the global.yaml configuration
func (l *ConfigLoader) loadGlobalConfig() error {
	configFile := filepath.Join(l.ConfigPath, "global.yaml")

	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read global.yaml from %s: %w", configFile, err)
	}

	l.GlobalConfig = &GlobalConfig{}
	if err := yaml.Unmarshal(data, l.GlobalConfig); err != nil {
		return fmt.Errorf("failed to parse global.yaml: %w", err)
	}

	return nil
}

// loadResourceConfig loads a resource configuration file
func (l *ConfigLoader) loadResourceConfig(resourceName string) error {
	configFile := filepath.Join(l.ConfigPath, fmt.Sprintf("%s.yaml", resourceName))

	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read %s.yaml from %s: %w", resourceName, configFile, err)
	}

	config := &ResourceConfig{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse %s.yaml: %w", resourceName, err)
	}

	l.ResourceConfigs[resourceName] = config
	return nil
}

// GetResourceConfig returns the configuration for a resource
func (l *ConfigLoader) GetResourceConfig(resourceName string) (*ResourceConfig, error) {
	config, ok := l.ResourceConfigs[resourceName]
	if !ok {
		return nil, fmt.Errorf("resource config not found: %s", resourceName)
	}
	return config, nil
}

// ParseFieldMappings parses the field_mapping section for a cloud
func ParseFieldMappings(fieldMapping map[string]interface{}) map[string][]CloudFieldMapping {
	result := make(map[string][]CloudFieldMapping)

	for terraformField, value := range fieldMapping {
		switch v := value.(type) {
		case []interface{}:
			var mappings []CloudFieldMapping
			for _, item := range v {
				if str, ok := item.(string); ok {
					mapping := parseFieldMappingString(str)
					if mapping != nil {
						mappings = append(mappings, *mapping)
					}
				}
			}
			if len(mappings) > 0 {
				result[terraformField] = mappings
			}
		}
	}

	return result
}

// parseFieldMappingString parses a field mapping string like "azure.properties.displayName (c*r*u)"
func parseFieldMappingString(s string) *CloudFieldMapping {
	// Pattern: "cloud.path (ops)" or "cloud.path|regex (ops)"
	re := regexp.MustCompile(`^([a-z]+\.[^\s\|]+)(?:\|([^\s]+))?\s*\(([cruCRUD\*]+)\)$`)
	matches := re.FindStringSubmatch(s)

	if len(matches) == 0 {
		return nil
	}

	mapping := &CloudFieldMapping{
		APIPath:    matches[1],
		Regex:      matches[2],
		Operations: strings.ToLower(matches[3]),
	}

	mapping.IsRequired = strings.Contains(mapping.Operations, "*")
	mapping.IsMaster = strings.Contains(mapping.Operations, "r*")

	return mapping
}

// BuildURL builds a URL from template and replaces placeholders
func BuildURL(template string, vars map[string]string) string {
	result := template
	for key, value := range vars {
		placeholder := fmt.Sprintf("{%s}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// ExtractFromResponse extracts a value from JSON response using JSONPath-like syntax
func ExtractFromResponse(data map[string]interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			return current[part], nil
		}

		next, ok := current[part]
		if !ok {
			return nil, fmt.Errorf("path not found: %s", path)
		}

		current, ok = next.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("intermediate path is not an object: %s", part)
		}
	}

	return nil, fmt.Errorf("empty path")
}

// ExtractWithRegex extracts a value from a string using regex
func ExtractWithRegex(input, pattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex: %w", err)
	}

	matches := re.FindStringSubmatch(input)
	if len(matches) < 2 {
		return "", fmt.Errorf("no match found")
	}

	// Return first capture group
	return matches[1], nil
}

// ParseMethodAndURL parses a string like "PUT https://..." into method and URL
func ParseMethodAndURL(methodURL string) (method, url string) {
	parts := strings.SplitN(methodURL, " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// If no method specified, assume GET
	return "GET", methodURL
}

// GetAbsoluteConfigPath returns the absolute path to the config directory
func GetAbsoluteConfigPath() (string, error) {
	// Try to find config directory relative to executable or working directory
	candidates := []string{
		"config",
		"../config",
		"../../config",
		filepath.Join(os.Getenv("HOME"), ".terraform-provider-omc", "config"),
	}

	for _, candidate := range candidates {
		absPath, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}

		if info, err := os.Stat(absPath); err == nil && info.IsDir() {
			return absPath, nil
		}
	}

	return "", fmt.Errorf("config directory not found")
}
