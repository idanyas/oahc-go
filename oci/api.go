package oci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/idanyas/oahc-go/config"
)

// Client for OCI API.
type Client struct {
	cfg        *config.Config
	signer     *Signer
	httpClient *http.Client
}

// NewClient creates a new OCI API client.
func NewClient(cfg *config.Config, signer *Signer) *Client {
	return &Client{
		cfg:    cfg,
		signer: signer,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
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
