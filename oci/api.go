package oci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/idanyas/oahc-go/config"
)

// The target interval between requests to stay under 3 requests/minute.
const requestInterval = 20 * time.Second

// Client for OCI API.
type Client struct {
	cfg             *config.Config
	signer          *Signer
	httpClient      *http.Client
	lastRequestTime time.Time
	pacerMutex      sync.Mutex
}

// NewClient creates a new OCI API client.
func NewClient(cfg *config.Config, signer *Signer) *Client {
	return &Client{
		cfg:        cfg,
		signer:     signer,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		// Initialize lastRequestTime to a time in the past to allow the first request immediately.
		lastRequestTime: time.Now().Add(-requestInterval),
	}
}

// paceRequest ensures that requests are spaced out to avoid hitting rate limits.
// It enforces a maximum of ~3 requests per minute.
func (c *Client) paceRequest() {
	c.pacerMutex.Lock()
	defer c.pacerMutex.Unlock()

	elapsed := time.Since(c.lastRequestTime)
	if elapsed < requestInterval {
		// Calculate how long to sleep.
		sleepDuration := requestInterval - elapsed
		// Add a small random jitter (0-2s) to avoid predictable patterns.
		jitter := time.Duration(rand.Intn(2000)) * time.Millisecond
		time.Sleep(sleepDuration + jitter)
	}
	// Mark the time of the current request.
	c.lastRequestTime = time.Now()
}

// APIError represents a structured error from the OCI API.
type APIError struct {
	StatusCode int
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("OCI API Error (status %d): %s - %s", e.StatusCode, e.Code, e.Message)
}

func (c *Client) buildAndDo(method, path string, queryParams url.Values, body interface{}) ([]byte, error) {
	// Proactively wait to ensure we comply with rate limits before making the call.
	c.paceRequest()

	baseURL := fmt.Sprintf("https://iaas.%s.oraclecloud.com/20160918", c.cfg.Region)
	if path == "/availabilityDomains/" {
		baseURL = fmt.Sprintf("https://identity.%s.oraclecloud.com/20160918", c.cfg.Region)
	}

	fullURL, err := url.Parse(baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	if queryParams != nil {
		fullURL.RawQuery = queryParams.Encode()
	}

	var reqBody []byte
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequest(method, fullURL.String(), bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.signer.Sign(req, reqBody); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// If configured, log specific API responses to a file.
	// We log all instance creation attempts, and all other failed API calls.
	if c.cfg.JSONLogPath != "" {
		isCreateInstance := method == http.MethodPost && path == "/instances/"
		isFailedResponse := resp.StatusCode < 200 || resp.StatusCode >= 300

		if isCreateInstance || isFailedResponse {
			go logResponseToFile(c.cfg.JSONLogPath, resp.Request.Method, resp.Request.URL.String(), resp.StatusCode, respBody)
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		// Try to unmarshal into the structured error format
		if json.Unmarshal(respBody, apiErr) == nil {
			return nil, apiErr
		}
		// If unmarshal fails, return a generic error
		apiErr.Message = string(respBody)
		return nil, apiErr
	}

	return respBody, nil
}

// logResponseToFile appends the details of an API response to the specified log file.
func logResponseToFile(path, method, url string, statusCode int, body []byte) {
	// Ensure the directory exists before trying to write the file.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Warning: could not create log directory %s: %v", dir, err)
		return
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: could not open API log file %s: %v", path, err)
		return
	}
	defer file.Close()

	var prettyBody bytes.Buffer
	logEntry := ""
	if json.Indent(&prettyBody, body, "", "  ") == nil {
		logEntry = prettyBody.String()
	} else {
		logEntry = string(body) // Fallback for non-JSON content
	}

	logLine := fmt.Sprintf("--- %s ---\n[%s] %s | Status: %d\n%s\n\n",
		time.Now().Format(time.RFC3339),
		method,
		url,
		statusCode,
		logEntry,
	)

	if _, err := file.WriteString(logLine); err != nil {
		log.Printf("Warning: failed to write to API log file %s: %v", path, err)
	}
}

// ListInstances fetches the list of compute instances.
func (c *Client) ListInstances() ([]Instance, error) {
	params := url.Values{}
	params.Add("compartmentId", c.cfg.TenancyID)

	respBody, err := c.buildAndDo(http.MethodGet, "/instances/", params, nil)
	if err != nil {
		return nil, err
	}

	var instances []Instance
	if err := json.Unmarshal(respBody, &instances); err != nil {
		return nil, fmt.Errorf("failed to unmarshal instances response: %w", err)
	}
	return instances, nil
}

// ListAvailabilityDomains fetches the list of availability domains.
func (c *Client) ListAvailabilityDomains() ([]AvailabilityDomain, error) {
	params := url.Values{}
	params.Add("compartmentId", c.cfg.TenancyID)

	respBody, err := c.buildAndDo(http.MethodGet, "/availabilityDomains/", params, nil)
	if err != nil {
		return nil, err
	}

	var domains []AvailabilityDomain
	if err := json.Unmarshal(respBody, &domains); err != nil {
		return nil, fmt.Errorf("failed to unmarshal availability domains response: %w", err)
	}
	return domains, nil
}

// CreateInstance attempts to launch a new compute instance.
func (c *Client) CreateInstance(availabilityDomain string) (*Instance, error) {
	// Build SourceDetails based on config
	var sourceDetails map[string]interface{}
	if c.cfg.BootVolumeID != "" {
		sourceDetails = map[string]interface{}{
			"sourceType":   "bootVolume",
			"bootVolumeId": c.cfg.BootVolumeID,
		}
	} else {
		sourceDetails = map[string]interface{}{
			"sourceType": "image",
			"imageId":    c.cfg.ImageID,
		}
		if c.cfg.BootVolumeSizeGbs > 0 {
			sourceDetails["bootVolumeSizeInGBs"] = c.cfg.BootVolumeSizeGbs
		}
	}

	reqBody := CreateInstanceDetails{
		AvailabilityDomain: availabilityDomain,
		CompartmentID:      c.cfg.TenancyID,
		Shape:              c.cfg.Shape,
		DisplayName:        fmt.Sprintf("instance-%s", time.Now().Format("20060102-1504")),
		Metadata:           map[string]string{"ssh_authorized_keys": c.cfg.SSHKey},
		SourceDetails:      sourceDetails,
		CreateVnicDetails: &VnicDetails{
			SubnetID:               c.cfg.SubnetID,
			AssignPublicIP:         false,
			AssignPrivateDNSRecord: true,
		},
		ShapeConfig: &ShapeConfig{
			Ocpus:       c.cfg.OCPUs,
			MemoryInGBs: c.cfg.MemoryInGBs,
		},
	}

	respBody, err := c.buildAndDo(http.MethodPost, "/instances/", nil, reqBody)
	if err != nil {
		return nil, err
	}

	var instance Instance
	if err := json.Unmarshal(respBody, &instance); err != nil {
		return nil, fmt.Errorf("failed to unmarshal create instance response: %w", err)
	}

	return &instance, nil
}
