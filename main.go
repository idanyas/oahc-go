package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/idanyas/oahc-go/backoff"
	"github.com/idanyas/oahc-go/config"
	"github.com/idanyas/oahc-go/notifier"
	"github.com/idanyas/oahc-go/oci"
)

func main() {
	envFile := flag.String("envfile", ".env", "Path to the environment file")
	flag.Parse()

	log.Println("Starting OCI Capacity Finder...")

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
	backoffManager := backoff.NewManager(cfg)

	// Main infinite loop to continuously check for capacity
	for {
		backoffManager.Wait()

		instances, err := client.ListInstances()
		if err != nil {
			log.Printf("ERROR: Failed to list instances: %v. Retrying in 30s...", err)
			time.Sleep(30 * time.Second)
			continue
		}

		existingInstances := 0
		for _, instance := range instances {
			if instance.Shape == cfg.Shape && instance.LifecycleState != "TERMINATED" {
				existingInstances++
			}
		}

		if existingInstances >= cfg.MaxInstances {
			log.Printf("Target instance count (%d) reached. Exiting.", cfg.MaxInstances)
			return
		}

		availabilityDomains, err := getAvailabilityDomains(client, cfg)
		if err != nil {
			log.Printf("ERROR: Failed to get availability domains: %v. Retrying in 30s...", err)
			time.Sleep(30 * time.Second)
			continue
		}

		tmrHitInCycle := false
		for _, ad := range availabilityDomains {
			instanceDetails, err := client.CreateInstance(ad)
			if err != nil {
				var apiErr *oci.APIError
				if errors.As(err, &apiErr) {
					if apiErr.StatusCode == 429 || apiErr.Code == "TooManyRequests" {
						log.Printf("Checking %s: Too Many Requests.", ad)
						tmrHitInCycle = true
						backoffManager.HandleTMR()
						break
					}
					if apiErr.StatusCode == 500 && strings.Contains(apiErr.Message, "Out of host capacity") {
						log.Printf("Checking %s: Out of capacity.", ad)
						continue
					}
				}
				log.Printf("Checking %s: Unrecoverable API Error: %v", ad, err)
				tmrHitInCycle = true
				break
			}

			// --- SUCCESS ---
			log.Printf("Checking %s: Success! Instance created.", ad)
			prettyDetails, _ := json.MarshalIndent(instanceDetails, "", "  ")

			// Full message for local log
			logSuccessMessage := fmt.Sprintf("Successfully created instance!\n%s", string(prettyDetails))
			log.Println(logSuccessMessage)

			// Send notification (JSON only)
			if cfg.TelegramBotAPIKey != "" && cfg.TelegramUserID != "" {
				telegramMessage := string(prettyDetails)
				tgNotifier := notifier.NewTelegramNotifier(cfg.TelegramBotAPIKey, cfg.TelegramUserID)
				if err := tgNotifier.Notify(telegramMessage); err != nil {
					log.Printf("Warning: failed to send Telegram notification: %v", err)
				} else {
					log.Println("Successfully sent Telegram notification.")
				}
			}

			backoffManager.Reset()
			return
		}

		// After trying all ADs, reset backoff if no TMR was hit.
		if !tmrHitInCycle {
			backoffManager.Reset()
			// Add a minimal pacing delay to prevent tight-looping when capacity is simply unavailable.
			time.Sleep(2 * time.Second)
		}
	}
}

func getAvailabilityDomains(client *oci.Client, cfg *config.Config) ([]string, error) {
	if cfg.AvailabilityDomain != "" {
		if strings.HasPrefix(cfg.AvailabilityDomain, "[") {
			var ads []string
			if err := json.Unmarshal([]byte(cfg.AvailabilityDomain), &ads); err != nil {
				return nil, fmt.Errorf("failed to parse OCI_AVAILABILITY_DOMAIN as JSON array: %w", err)
			}
			return ads, nil
		}
		return []string{cfg.AvailabilityDomain}, nil
	}

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
