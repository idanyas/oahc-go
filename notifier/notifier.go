package notifier

// Notifier defines the interface for sending notifications.
type Notifier interface {
	Notify(message string) error
}
