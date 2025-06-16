package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/idanyas/oahc-go/config"
	"github.com/idanyas/oahc-go/notifier"
	"github.com/idanyas/oahc-go/oci"
)

const tooManyRequestsWaiterFile = "too_many_requests_waiter.txt"

func main() {
	envFile := flag.String("envfile", ".env", "Path to the environment file")
	flag.Parse()

	cfg, err := config.Load(*envFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	signer, err := oci.NewSigner(cfg.TenancyID, cfg.UserID, cfg.KeyFingerprint, cfg.PrivateKeyPath)
	if err != nil {
		log.Fatalf("Failed to create OCI signer: %v", err)
	}

	client := oci.NewClient(cfg, signer)

	// Handle "Too Many Requests" waiter logic
	if err := checkWaiter(); err != nil {
		log.Println(err)
		return
	}

	instances, err := client.ListInstances()
	if err != nil {
		log.Fatalf("Failed to list instances: %v", err)
	}

	existingInstances := 0
	for _, instance := range instances {
		if instance.Shape == cfg.Shape && instance.LifecycleState != "TERMINATED" {
			existingInstances++
		}
	}

	if existingInstances >= cfg.MaxInstances {
		log.Printf("Already have %d instance(s) of shape %s. Maximum is %d. Exiting.", existingInstances, cfg.Shape, cfg.MaxInstances)
		return
	}

	log.Println("Starting search for available capacity...")

	availabilityDomains, err := getAvailabilityDomains(client, cfg)
	if err != nil {
		log.Fatalf("Failed to get availability domains: %v", err)
	}

	for _, ad := range availabilityDomains {
		log.Printf("Trying Availability Domain: %s", ad)
		instanceDetails, err := client.CreateInstance(ad)
		if err != nil {
			var apiErr *oci.APIError
			if errors.As(err, &apiErr) {
				// Specific OCI API error
				if apiErr.StatusCode == 500 && strings.Contains(apiErr.Message, "Out of host capacity") {
					log.Printf("Out of host capacity in %s. Trying next...", ad)
					time.Sleep(16 * time.Second) // Mimic original script's sleep
					continue
				}
				if apiErr.StatusCode == 429 || apiErr.Code == "TooManyRequests" {
					log.Printf("Too many requests, backing off for %d seconds. Error: %s", cfg.TooManyRequestsWait, apiErr.Message)
					if err := setWaiter(cfg.TooManyRequestsWait); err != nil {
						log.Printf("Warning: failed to set waiter file: %v", err)
					}
					// Exit because we should wait before the next run.
					return
				}
			}
			// For other errors, exit as it's likely a config issue
			log.Fatalf("Failed to create instance in %s: %v", ad, err)
		}

		// Success!
		prettyDetails, _ := json.MarshalIndent(instanceDetails, "", "  ")
		successMessage := fmt.Sprintf("Successfully created instance!\n%s", string(prettyDetails))
		log.Println(successMessage)

		// Send notification if configured
		if cfg.TelegramBotAPIKey != "" && cfg.TelegramUserID != "" {
			tgNotifier := notifier.NewTelegramNotifier(cfg.TelegramBotAPIKey, cfg.TelegramUserID)
			if err := tgNotifier.Notify(successMessage); err != nil {
				log.Printf("Warning: failed to send Telegram notification: %v", err)
			} else {
				log.Println("Successfully sent Telegram notification.")
			}
		}

		// We are done, remove waiter file if it exists and exit
		_ = removeWaiter()
		return
	}

	log.Println("No capacity found in any of the checked availability domains.")
}

func getAvailabilityDomains(client *oci.Client, cfg *config.Config) ([]string, error) {
	if cfg.AvailabilityDomain != "" {
		// OCI_AVAILABILITY_DOMAIN can be a single string or a JSON array of strings
		if strings.HasPrefix(cfg.AvailabilityDomain, "[") {
			var ads []string
			if err := json.Unmarshal([]byte(cfg.AvailabilityDomain), &ads); err != nil {
				return nil, fmt.Errorf("failed to parse OCI_AVAILABILITY_DOMAIN as JSON array: %w", err)
			}
			return ads, nil
		}
		return []string{cfg.AvailabilityDomain}, nil
	}

	log.Println("OCI_AVAILABILITY_DOMAIN not set, fetching list from OCI...")
	ociAds, err := client.ListAvailabilityDomains()
	if err != nil {
		return nil, err
	}
	var adNames []string
	for _, ad := range ociAds {
		adNames = append(adNames, ad.Name)
	}
	return adNames, nil
}

func getWaiterFilePath() string {
	return filepath.Join(os.TempDir(), tooManyRequestsWaiterFile)
}

func checkWaiter() error {
	path := getWaiterFilePath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil // File doesn't exist, we can proceed
	}
	if err != nil {
		return fmt.Errorf("could not read waiter file: %w", err)
	}

	waitUntil, err := time.Parse(time.RFC3339, string(data))
	if err != nil {
		// If file is corrupt, remove it and proceed
		_ = removeWaiter()
		return nil
	}

	if time.Now().Before(waitUntil) {
		return fmt.Errorf("waiter is active, will not run until %s (in %s)", waitUntil.Format(time.Kitchen), time.Until(waitUntil).Round(time.Second))
	}

	// Wait time has passed, remove the file
	_ = removeWaiter()
	return nil
}

func setWaiter(waitSeconds int) error {
	path := getWaiterFilePath()
	waitUntil := time.Now().Add(time.Duration(waitSeconds) * time.Second)
	return os.WriteFile(path, []byte(waitUntil.Format(time.RFC3339)), 0644)
}

func removeWaiter() error {
	path := getWaiterFilePath()
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
