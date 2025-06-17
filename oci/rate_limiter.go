package oci

import (
	"sync"
	"time"
)

const (
	longTermWindow  = 60 * time.Second
	longTermLimit   = 10
	shortTermWindow = 10 * time.Second
	shortTermLimit  = 4
)

// RateLimiter enforces OCI API request rate limits proactively.
type RateLimiter struct {
	requestTimestamps []time.Time
	mutex             sync.Mutex
}

// NewRateLimiter creates a new proactive rate limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		requestTimestamps: make([]time.Time, 0),
	}
}

// Wait blocks until a new request can be made without violating rate limits.
func (rl *RateLimiter) Wait() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	for {
		now := time.Now()

		// 1. Prune old timestamps (older than the longest window).
		cutoff := now.Add(-longTermWindow)
		n := 0
		for _, ts := range rl.requestTimestamps {
			if ts.After(cutoff) {
				rl.requestTimestamps[n] = ts
				n++
			}
		}
		rl.requestTimestamps = rl.requestTimestamps[:n]

		// 2. Check the rules.
		longTermCount := len(rl.requestTimestamps)

		shortTermCutoff := now.Add(-shortTermWindow)
		shortTermCount := 0
		for _, ts := range rl.requestTimestamps {
			if ts.After(shortTermCutoff) {
				shortTermCount++
			}
		}

		// 3. If limits are hit, calculate sleep time and wait.
		if longTermCount >= longTermLimit || shortTermCount >= shortTermLimit {
			var sleepDuration time.Duration

			// Calculate time until the oldest request in the long window expires.
			if longTermCount >= longTermLimit {
				oldestRelevant := rl.requestTimestamps[longTermCount-longTermLimit]
				sleepDuration = oldestRelevant.Add(longTermWindow).Sub(now) + time.Millisecond // Add a millisecond for safety
			}

			// Calculate time until the oldest request in the short window expires.
			if shortTermCount >= shortTermLimit {
				oldestRelevant := rl.requestTimestamps[longTermCount-shortTermLimit]
				shortSleep := oldestRelevant.Add(shortTermWindow).Sub(now) + time.Millisecond
				// If this sleep is shorter, it's the one we need.
				if sleepDuration == 0 || shortSleep < sleepDuration {
					sleepDuration = shortSleep
				}
			}

			if sleepDuration > 0 {
				time.Sleep(sleepDuration)
			}
			continue // Re-check conditions after sleeping
		}

		// 4. It's safe to proceed, record the new request and exit the loop.
		rl.requestTimestamps = append(rl.requestTimestamps, now)
		break
	}
}