package logger

import (
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

type RelayLogger struct {
	consoleLogger *log.Logger
	fileLogger    *log.Logger
}

func (rl *RelayLogger) log(level log.Level, msg string, keyvals ...interface{}) {
	switch level {
	case log.DebugLevel:
		rl.consoleLogger.Debug(msg, keyvals...)
		rl.fileLogger.Debug(msg, keyvals...)
	case log.InfoLevel:
		rl.consoleLogger.Info(msg, keyvals...)
		rl.fileLogger.Info(msg, keyvals...)
	case log.WarnLevel:
		rl.consoleLogger.Warn(msg, keyvals...)
		rl.fileLogger.Warn(msg, keyvals...)
	case log.ErrorLevel:
		rl.consoleLogger.Error(msg, keyvals...)
		rl.fileLogger.Error(msg, keyvals...)
	case log.FatalLevel:
		rl.consoleLogger.Fatal(msg, keyvals...)
		rl.fileLogger.Fatal(msg, keyvals...)
	}
}

func (rl *RelayLogger) Info(msg string, keyvals ...interface{}) {
	rl.log(log.InfoLevel, msg, keyvals...)
}
func (rl *RelayLogger) Error(msg string, keyvals ...interface{}) {
	rl.log(log.ErrorLevel, msg, keyvals...)
}
func (rl *RelayLogger) Debug(msg string, keyvals ...interface{}) {
	rl.log(log.DebugLevel, msg, keyvals...)
}
func (rl *RelayLogger) Warn(msg string, keyvals ...interface{}) {
	rl.log(log.WarnLevel, msg, keyvals...)
}
func (rl *RelayLogger) Fatal(msg string, keyvals ...interface{}) {
	rl.log(log.FatalLevel, msg, keyvals...)
}

func (rl *RelayLogger) SetLevel(level log.Level) {
	rl.consoleLogger.SetLevel(level)
	rl.fileLogger.SetLevel(level)
}

func (rl *RelayLogger) ConnectingToRelay(relayURL string) {
	rl.Info("Connecting to relay",
		"relay_url", relayURL,
	)
}

func (rl *RelayLogger) ConnectionLost(relayURL string) {
	rl.Error("Connection to rleay lost",
		"relay_url", relayURL)
}

func (rl *RelayLogger) ConnectionReestablished(relayURL string) {
	rl.Info("Connection reestablished",
		"relay_url", relayURL)
}

func (rl *RelayLogger) FailureToConnectToRelay(relayURL string, err error) {
	rl.Error("Failed to connect to relay",
		"relay_url", relayURL,
		"error", err)
}

func (rl *RelayLogger) FailureToPublishEvent(relayURL string, err error) {
	rl.Error("Publishing event failed",
		"relay_url", relayURL,
		"error", err,
	)
}

func (rl *RelayLogger) RelayConnected(relayURL string) {
	rl.Info("Relay connected",
		"relay_url", relayURL,
	)
}

func (rl *RelayLogger) RelayDisconnected(relayURL string) {
	rl.Info("Relay disconnected",
		"relay_url", relayURL,
	)
}

func (rl *RelayLogger) EventReceived(relayURL, eventID string) {
	rl.Info("Event received",
		"relay_url", relayURL,
		"event_id", eventID,
	)
}

func (rl *RelayLogger) EventPublished(relayURL, eventID string) {
	rl.Info("Event published",
		"relay_url", relayURL,
		"event_id", eventID,
	)
}

func (rl *RelayLogger) SubscriptionCreated(relayURL string) {
	rl.Info("Subscription created",
		"relay_url", relayURL,
	)
}

func (rl *RelayLogger) SubscriptionFailed(relayURL string, err error) {
	rl.Error("Failed to subscribe",
		"relay_url", relayURL,
		"error", err,
	)
}

func (rl *RelayLogger) TimestampsSaved(count int) {
	rl.Info("Timestamps saved to disk",
		"relay_count", count,
	)
}

func (rl *RelayLogger) TimestampsLoaded(count int) {
	rl.Info("Timestamps loaded from disk",
		"relay_count", count,
	)
}

func (rl *RelayLogger) HistoricalSync(relayURL string, fromTime time.Time) {
	rl.Info("Historical sync starting",
		"relay_url", relayURL,
		"from_time", fromTime.Format(time.RFC3339),
	)
}

func NewRelayLogger() (*RelayLogger, error) {
	logFile, err := os.OpenFile("log.json", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

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

	consoleLogger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
		Prefix:          "🌎 nostr-relay ",
		Level:           log.InfoLevel,
	})

	fileLogger := log.NewWithOptions(logFile, log.Options{
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
		Level:           log.InfoLevel,
		Formatter:       log.JSONFormatter,
	})

	consoleLogger.SetStyles(styles)

	return &RelayLogger{
		consoleLogger: consoleLogger,
		fileLogger:    fileLogger,
	}, nil
}
