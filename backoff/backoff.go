package backoff

import (
	"log"
	"time"

	"github.com/idanyas/oahc-go/config"
)

// Manager handles in-memory exponential backoff state.
type Manager struct {
	initialDelay time.Duration
	maxDelay     time.Duration
	currentDelay time.Duration
	waitUntil    time.Time
}

// NewManager creates a new backoff state manager.
func NewManager(cfg *config.Config) *Manager {
	initial := time.Duration(cfg.BackoffInitialSeconds) * time.Second
	return &Manager{
		initialDelay: initial,
		maxDelay:     time.Duration(cfg.BackoffMaxSeconds) * time.Second,
		currentDelay: initial,
		waitUntil:    time.Now(),
	}
}

// Wait checks the backoff state and sleeps if necessary.
func (m *Manager) Wait() {
	waitDuration := time.Until(m.waitUntil)
	if waitDuration > 0 {
		log.Printf("Backoff activated, sleeping for %v.", waitDuration.Round(time.Second))
		time.Sleep(waitDuration)
	}
}

// HandleTMR is called when a "Too Many Requests" error occurs.
// It doubles the delay and sets the wait time for the next cycle.
func (m *Manager) HandleTMR() {
	// Set the time to wait based on the *current* delay
	m.waitUntil = time.Now().Add(m.currentDelay)

	// Calculate the next delay for the *next* time TMR is hit.
	m.currentDelay *= 2
	if m.currentDelay > m.maxDelay {
		m.currentDelay = m.maxDelay
	}
}

// Reset brings the backoff state to its initial aggressive setting.
func (m *Manager) Reset() {
	if m.currentDelay > m.initialDelay {
		// log.Println("Resetting backoff state to be aggressive.")
		m.currentDelay = m.initialDelay
	}
	m.waitUntil = time.Now()
}
