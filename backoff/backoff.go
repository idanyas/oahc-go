package backoff

import (
	"log"
	"time"

	"github.com/idanyas/oahc-go/config"
)

// Manager handles the stateful backoff logic after a 429 error.
type Manager struct {
	lastWasTMR bool
}

// NewManager creates a new backoff state manager.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		lastWasTMR: false,
	}
}

// HandleTMR is called when a "Too Many Requests" error occurs.
// It sleeps for 20s on the first TMR, and 40s on subsequent consecutive TMRs.
func (m *Manager) HandleTMR() {
	var sleepDuration time.Duration

	if m.lastWasTMR {
		// This is a consecutive 429, back off for 40s.
		sleepDuration = 40 * time.Second
	} else {
		// This is the first 429 in a sequence, back off for 20s.
		sleepDuration = 20 * time.Second
	}

	log.Printf("Backoff activated, sleeping for %v.", sleepDuration)
	time.Sleep(sleepDuration)

	// Set the state for the next potential error.
	m.lastWasTMR = true
}

// Reset clears the backoff state, ensuring the next TMR uses the initial 20s wait.
// This should be called after any successful API call or a full loop without a TMR.
func (m *Manager) Reset() {
	m.lastWasTMR = false
}