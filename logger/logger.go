package logger

import (
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

type RelayLogger struct {
	*log.Logger
}

func (rl *RelayLogger) ConnectingToRelay(relayURL string) {
	rl.Info("Connecting to relay",
		"relay_url", relayURL,
	)
}

func (rl *RelayLogger) FailureToConnectToRelay(relayURL string, err error) {
	rl.Error("Failed to connect to relay",
		"relay_url", relayURL,
		"error", err)
}

func (rl *RelayLogger) RelayConnected(relayURL string) {
	rl.Info("Relay connected",
		"relay_url", relayURL,
	)
}

func NewRelayLogger() *RelayLogger {
	styles := log.DefaultStyles()

	// Green for connections
	styles.Keys["connection"] = lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).Bold(true)
	styles.Values["connection"] = lipgloss.NewStyle().
		Foreground(lipgloss.Color("10"))

	// Red for errors
	styles.Keys["error"] = lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")).Bold(true)
	styles.Values["error"] = lipgloss.NewStyle().
		Foreground(lipgloss.Color("9"))

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
		Prefix:          "🌎 nostr-relay",
		Level:           log.InfoLevel,
	})

	logger.SetStyles(styles)

	return &RelayLogger{Logger: logger}
}
