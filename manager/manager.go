package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.crom/crbroughton/townsquares-relay/logger"
)

type RelayConnection struct {
	URL                string
	Relay              *nostr.Relay
	active             bool
	mu                 sync.RWMutex
	lastSeenTimestamp  time.Time
	lastDisconnectTime time.Time
	isFirstConnection  bool
}

type EventMetadata struct {
	SourceRelay string
	ReceivedAt  time.Time
	Local       bool
}

type RelayTimestamps struct {
	Timestamps map[string]time.Time `json:"timestamps"`
	LastSaved  time.Time            `json:"last_saved"`
}

type RelayManager struct {
	connections         map[string]*RelayConnection
	mu                  sync.RWMutex
	eventStore          map[string]*nostr.Event
	eventMetadata       map[string]*EventMetadata
	storeMu             sync.RWMutex
	seenEvents          map[string]bool
	seenMu              sync.RWMutex
	logger              *logger.RelayLogger
	tailscaleClient     *http.Client
	timestampFilePath   string
	savedTimestamps     map[string]time.Time
	timestampMu         sync.RWMutex
	historicalSyncRelay string // URL of relay currently providing historical sync
	historicalSyncMu    sync.RWMutex
	shutdownChan        chan struct{}
	timestampSaveMu     sync.Mutex // Protects timestamp save operations
}

func NewRelayManager() *RelayManager {
	logger, err := logger.NewRelayLogger()
	if err != nil {
		panic(err)
	}
	rm := &RelayManager{
		connections:       make(map[string]*RelayConnection),
		eventStore:        make(map[string]*nostr.Event),
		eventMetadata:     make(map[string]*EventMetadata),
		seenEvents:        make(map[string]bool),
		logger:            logger,
		tailscaleClient:   http.DefaultClient,
		timestampFilePath: "relay_timestamps.json",
		savedTimestamps:   make(map[string]time.Time),
		shutdownChan:      make(chan struct{}),
	}
	rm.loadTimestamps()

	// Start background timestamp saver
	go rm.startTimestampSaver()

	return rm
}

func (rm *RelayManager) SetTailscaleClient(client *http.Client) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.tailscaleClient = client
}

func (rm *RelayManager) Connect(ctx context.Context, url string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.connections[url]; exists {
		// We're already connected to this relay
		return nil
	}

	rm.logger.ConnectingToRelay(url)
	relay, err := nostr.RelayConnect(ctx, url)
	if err != nil {
		rm.logger.FailureToConnectToRelay(url, err)
		return fmt.Errorf("failed to connect to relay %s: %w", url, err)
	}

	// Check if we have a saved timestamp for this relay
	rm.timestampMu.RLock()
	savedTimestamp, hasTimestamp := rm.savedTimestamps[url]
	rm.timestampMu.RUnlock()

	conn := &RelayConnection{
		URL:               url,
		Relay:             relay,
		active:            true,
		lastSeenTimestamp: time.Now(),
		isFirstConnection: !hasTimestamp,
	}

	if hasTimestamp {
		conn.lastSeenTimestamp = savedTimestamp
	}

	rm.connections[url] = conn
	rm.logger.RelayConnected(url)

	go rm.Subscribe(ctx, conn)

	return nil
}

func (rm *RelayManager) handleIncomingEvent(event *nostr.Event, sourceURL string) {
	// Make sure no dupes
	rm.seenMu.Lock()
	if rm.seenEvents[event.ID] {
		rm.seenMu.Unlock()
		return
	}
	rm.seenEvents[event.ID] = true
	rm.seenMu.Unlock()

	now := time.Now()
	rm.storeMu.Lock()
	rm.eventStore[event.ID] = event
	rm.eventMetadata[event.ID] = &EventMetadata{
		SourceRelay: sourceURL,
		ReceivedAt:  now,
		Local:       false,
	}
	rm.storeMu.Unlock()

	// Update timestamp for this relay connection
	rm.mu.RLock()
	if conn, exists := rm.connections[sourceURL]; exists {
		conn.mu.Lock()
		conn.lastSeenTimestamp = now
		conn.mu.Unlock()
	}
	rm.mu.RUnlock()

	rm.logger.EventReceived(sourceURL, event.ID[:8])
}

func (rm *RelayManager) Subscribe(ctx context.Context, conn *RelayConnection) {
	backoff := 5 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Build filter based on connection state and coordination logic
		filter := nostr.Filter{
			Kinds: []int{nostr.KindTextNote},
		}

		conn.mu.RLock()
		isFirst := conn.isFirstConnection
		lastSeen := conn.lastSeenTimestamp
		conn.mu.RUnlock()

		// Determine if this relay should provide historical sync
		rm.historicalSyncMu.Lock()
		shouldDoHistoricalSync := rm.historicalSyncRelay == "" && !isFirst
		if shouldDoHistoricalSync {
			rm.historicalSyncRelay = conn.URL
		}
		rm.historicalSyncMu.Unlock()

		if isFirst {
			// First connection - get recent events
			filter.Limit = 100
		} else if shouldDoHistoricalSync {
			// This relay will provide historical sync
			since := nostr.Timestamp(lastSeen.Unix())
			filter.Since = &since
			filter.Limit = 1000 // Higher limit for historical sync
			rm.logger.HistoricalSync(conn.URL, lastSeen)
		} else {
			// Real-time only - another relay is handling historical sync
			filter.Limit = 0 // Real-time only
		}

		sub, err := conn.Relay.Subscribe(ctx, []nostr.Filter{filter})
		if err != nil {
			rm.logger.SubscriptionFailed(conn.URL, err)
			conn.mu.Lock()
			conn.active = false
			conn.mu.Unlock()

			time.Sleep(backoff)
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}

			if err := rm.reconnect(ctx, conn); err != nil {
				rm.logger.FailureToConnectToRelay(conn.URL, err)
				continue
			}
			backoff = 5 * time.Second // Reset backoff on successful reconnect
			continue
		}

		conn.mu.Lock()
		conn.active = true
		conn.mu.Unlock()
		backoff = 5 * time.Second // Reset backoff on successful subscribe

		for ev := range sub.Events {
			select {
			case <-ctx.Done():
				return
			default:
				rm.handleIncomingEvent(ev, conn.URL)
			}
		}

		rm.logger.ConnectionLost(conn.URL)
		conn.mu.Lock()
		conn.active = false
		conn.lastDisconnectTime = time.Now()
		conn.mu.Unlock()

		// Reset historical sync if this was the sync relay
		rm.historicalSyncMu.Lock()
		if rm.historicalSyncRelay == conn.URL {
			rm.historicalSyncRelay = ""
		}
		rm.historicalSyncMu.Unlock()

		// Save timestamps when connection is lost
		go rm.saveTimestamps()

		time.Sleep(backoff)
	}
}

func (rm *RelayManager) reconnect(ctx context.Context, conn *RelayConnection) error {
	if conn.Relay != nil {
		conn.Relay.Close()
	}

	relay, err := nostr.RelayConnect(ctx, conn.URL)
	if err != nil {
		return err
	}

	conn.mu.Lock()
	conn.Relay = relay
	conn.active = true
	conn.mu.Unlock()

	rm.logger.ConnectionReestablished(conn.URL)
	return nil
}

func (rm *RelayManager) Broadcast(ctx context.Context, event *nostr.Event) {
	// This relay has now seen this event
	rm.seenMu.Lock()
	rm.seenEvents[event.ID] = true
	rm.seenMu.Unlock()

	rm.storeMu.Lock()
	rm.eventMetadata[event.ID] = &EventMetadata{
		SourceRelay: "local",
		ReceivedAt:  time.Now(),
		Local:       true,
	}
	rm.storeMu.Unlock()

	// Now we broadcast to all the relays
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	for url, conn := range rm.connections {
		if !conn.active {
			continue
		}

		go func(relay *nostr.Relay, relayURL string) {
			if err := relay.Publish(ctx, *event); err != nil {
				rm.logger.FailureToPublishEvent(relayURL, err)
			} else {
				rm.logger.EventPublished(relayURL, event.ID[:8])
			}
		}(conn.Relay, url)
	}
}

func (rm *RelayManager) GetAllEvents() []*nostr.Event {
	rm.storeMu.RLock()
	defer rm.storeMu.RUnlock()

	events := make([]*nostr.Event, 0, len(rm.eventStore))
	for _, event := range rm.eventStore {
		events = append(events, event)
	}
	return events
}

func (rm *RelayManager) StartSubscriptions(ctx context.Context) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	for _, conn := range rm.connections {
		if conn.active {
			go rm.Subscribe(ctx, conn)
		}
	}
}

func (rm *RelayManager) Close() {
	// Signal shutdown to background saver (this will do a final save)
	close(rm.shutdownChan)

	rm.mu.Lock()
	defer rm.mu.Unlock()

	for url, conn := range rm.connections {
		conn.Relay.Close()
		rm.logger.RelayDisconnected(url)
	}
}

// loadTimestamps loads previously saved timestamps from disk
func (rm *RelayManager) loadTimestamps() {
	data, err := os.ReadFile(rm.timestampFilePath)
	if err != nil {
		// File doesn't exist or can't be read - this is fine for first run
		return
	}

	var timestamps RelayTimestamps
	if err := json.Unmarshal(data, &timestamps); err != nil {
		rm.logger.SubscriptionFailed("timestamp-loader", fmt.Errorf("failed to parse timestamp file: %w", err))
		return
	}

	// Store timestamps to be applied when connections are created
	rm.timestampMu.Lock()
	for url, timestamp := range timestamps.Timestamps {
		rm.savedTimestamps[url] = timestamp
	}
	rm.timestampMu.Unlock()

	rm.logger.TimestampsLoaded(len(rm.savedTimestamps))
}

// startTimestampSaver runs a background goroutine that saves timestamps every 5 seconds
func (rm *RelayManager) startTimestampSaver() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rm.saveTimestamps()
		case <-rm.shutdownChan:
			// Final save before shutdown
			rm.saveTimestamps()
			return
		}
	}
}

// saveTimestamps saves current relay timestamps to disk with mutex protection
func (rm *RelayManager) saveTimestamps() {
	rm.timestampSaveMu.Lock()
	defer rm.timestampSaveMu.Unlock()

	rm.mu.RLock()
	timestamps := RelayTimestamps{
		Timestamps: make(map[string]time.Time),
		LastSaved:  time.Now(),
	}

	for url, conn := range rm.connections {
		conn.mu.RLock()
		timestamps.Timestamps[url] = conn.lastSeenTimestamp
		conn.mu.RUnlock()
	}
	rm.mu.RUnlock()

	data, err := json.MarshalIndent(timestamps, "", "  ")
	if err != nil {
		rm.logger.SubscriptionFailed("timestamp-saver", fmt.Errorf("failed to marshal timestamps: %w", err))
		return
	}

	if err := os.WriteFile(rm.timestampFilePath, data, 0644); err != nil {
		rm.logger.SubscriptionFailed("timestamp-saver", fmt.Errorf("failed to save timestamps: %w", err))
	} else {
		rm.logger.TimestampsSaved(len(timestamps.Timestamps))
	}
}
