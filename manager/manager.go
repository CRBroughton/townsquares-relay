package manager

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.crom/crbroughton/townsquares-relay/logger"
)

type RelayConnection struct {
	URL    string
	Relay  *nostr.Relay
	active bool
}

type EventMetadata struct {
	SourceRelay string
	ReceivedAt  time.Time
	Local       bool
}

type RelayManager struct {
	connections   map[string]*RelayConnection
	mu            sync.RWMutex
	eventStore    map[string]*nostr.Event
	eventMetadata map[string]*EventMetadata
	storeMu       sync.RWMutex
	seenEvents    map[string]bool
	seenMu        sync.RWMutex
	logger        *logger.RelayLogger
}

func NewRelayManager() *RelayManager {
	logger := logger.NewRelayLogger()
	return &RelayManager{
		connections:   make(map[string]*RelayConnection),
		eventStore:    make(map[string]*nostr.Event),
		eventMetadata: make(map[string]*EventMetadata),
		seenEvents:    make(map[string]bool),
		logger:        logger,
	}
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
		return fmt.Errorf("failed to connect to the relay at %s: %w", url, err)

	}

	conn := &RelayConnection{
		URL:    url,
		Relay:  relay,
		active: true,
	}
	rm.connections[url] = conn
	rm.logger.RelayConnected(url)

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

	rm.storeMu.Lock()
	rm.eventStore[event.ID] = event
	rm.eventMetadata[event.ID] = &EventMetadata{
		SourceRelay: sourceURL,
		ReceivedAt:  time.Now(),
		Local:       false,
	}

	rm.storeMu.Unlock()
	rm.logger.EventReceived(sourceURL, event.ID[:8])
}

func (rm *RelayManager) Subscribe(ctx context.Context, conn *RelayConnection) {
	sub, err := conn.Relay.Subscribe(ctx, []nostr.Filter{
		{
			Kinds: []int{nostr.KindTextNote},
			Limit: 100,
		},
	})
	if err != nil {
		log.Printf("Failed to subscribe to the relay at %s: %v", conn.URL, err)
		return
	}
	rm.logger.SubscriptionCreated(conn.URL)

	for event := range sub.Events {
		rm.logger.EventReceived(conn.URL, event.ID[:8])
		rm.handleIncomingEvent(event, conn.URL)
	}
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
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for url, conn := range rm.connections {
		conn.Relay.Close()
		rm.logger.RelayDisconnected(url)
	}
}
