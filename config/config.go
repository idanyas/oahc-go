package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the application.
type Config struct {
	// OCI General
	Region         string
	UserID         string
	TenancyID      string
	KeyFingerprint string
	PrivateKeyPath string

	// Instance Parameters
	AvailabilityDomain string
	SubnetID           string
	ImageID            string
	Shape              string
	OCPUs              int
	MemoryInGBs        int
	SSHKey             string
	MaxInstances       int
	BootVolumeSizeGbs  int    // Optional
	BootVolumeID       string // Optional

	// Notifications
	TelegramBotAPIKey string
	TelegramUserID    string

	// App behavior
	TooManyRequestsWait int
}

// Load reads configuration from a .env file and environment variables.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	defaults(cfg)

	envMap, err := readEnvFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error reading env file %s: %w", path, err)
		}
		// It's okay if the file doesn't exist, we'll rely on environment variables.
		envMap = make(map[string]string)
	}

	// Helper to get value from file map or fallback to environment variable
	getValue := func(key string) string {
		if val, ok := envMap[key]; ok {
			return val
		}
		return os.Getenv(key)
	}

	cfg.Region = getValue("OCI_REGION")
	cfg.UserID = getValue("OCI_USER_ID")
	cfg.TenancyID = getValue("OCI_TENANCY_ID")
	cfg.KeyFingerprint = getValue("OCI_KEY_FINGERPRINT")
	cfg.PrivateKeyPath = getValue("OCI_PRIVATE_KEY_FILENAME")
	cfg.AvailabilityDomain = getValue("OCI_AVAILABILITY_DOMAIN")
	cfg.SubnetID = getValue("OCI_SUBNET_ID")
	cfg.ImageID = getValue("OCI_IMAGE_ID")
	cfg.Shape = getValue("OCI_SHAPE")
	cfg.SSHKey = getValue("OCI_SSH_PUBLIC_KEY")
	cfg.BootVolumeID = getValue("OCI_BOOT_VOLUME_ID")

	cfg.TelegramBotAPIKey = getValue("TELEGRAM_BOT_API_KEY")
	cfg.TelegramUserID = getValue("TELEGRAM_USER_ID")

	// Integer values
	if val := getValue("OCI_OCPUS"); val != "" {
		cfg.OCPUs, _ = strconv.Atoi(val)
	}
	if val := getValue("OCI_MEMORY_IN_GBS"); val != "" {
		cfg.MemoryInGBs, _ = strconv.Atoi(val)
	}
	if val := getValue("OCI_MAX_INSTANCES"); val != "" {
		cfg.MaxInstances, _ = strconv.Atoi(val)
	}
	if val := getValue("OCI_BOOT_VOLUME_SIZE_IN_GBS"); val != "" {
		cfg.BootVolumeSizeGbs, _ = strconv.Atoi(val)
	}
	if val := getValue("TOO_MANY_REQUESTS_TIME_WAIT"); val != "" {
		cfg.TooManyRequestsWait, _ = strconv.Atoi(val)
	}

	return cfg, nil
}

// Validate checks if the essential configuration values are set.
func (c *Config) Validate() error {
	required := map[string]string{
		"OCI_REGION":               c.Region,
		"OCI_USER_ID":              c.UserID,
		"OCI_TENANCY_ID":           c.TenancyID,
		"OCI_KEY_FINGERPRINT":      c.KeyFingerprint,
		"OCI_PRIVATE_KEY_FILENAME": c.PrivateKeyPath,
		"OCI_SUBNET_ID":            c.SubnetID,
		"OCI_SHAPE":                c.Shape,
		"OCI_SSH_PUBLIC_KEY":       c.SSHKey,
	}

	// Either ImageID or BootVolumeID must be present
	if c.ImageID == "" && c.BootVolumeID == "" {
		return fmt.Errorf("either OCI_IMAGE_ID or OCI_BOOT_VOLUME_ID must be set")
	}

	for key, val := range required {
		if val == "" {
			return fmt.Errorf("mandatory configuration %s is not set", key)
		}
	}

	if c.BootVolumeID != "" && c.BootVolumeSizeGbs > 0 {
		return fmt.Errorf("OCI_BOOT_VOLUME_ID and OCI_BOOT_VOLUME_SIZE_IN_GBS cannot be used together")
	}

	return nil
}

// defaults sets default values for the configuration.
func defaults(c *Config) {
	c.Shape = "VM.Standard.A1.Flex"
	c.OCPUs = 4
	c.MemoryInGBs = 24
	c.MaxInstances = 1
	c.TooManyRequestsWait = 300 // 5 minutes
}

// readEnvFile parses a .env file and returns a map of key-value pairs.
func readEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	envMap := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Unquote value if it's quoted
		if len(value) > 1 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		envMap[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return envMap, nil
}
