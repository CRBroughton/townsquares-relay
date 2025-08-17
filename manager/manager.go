package manager

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/nbd-wtf/go-nostr"
)

type RelayConnection struct {
	URL    string
	Relay  *nostr.Relay
	active bool
}

type RelayManager struct {
	connections map[string]*RelayConnection
	mu          sync.RWMutex
}

func NewRelayManager() *RelayManager {
	return &RelayManager{
		connections: make(map[string]*RelayConnection),
	}
}

func (rm *RelayManager) Connect(ctx context.Context, url string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.connections[url]; exists {
		// We're already connected to this relay
		return nil
	}

	log.Printf("Connecting to an external relay at: %s", url)
	relay, err := nostr.RelayConnect(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to connect to the relay at %s: %w", url, err)
	}

	conn := &RelayConnection{
		URL:    url,
		Relay:  relay,
		active: true,
	}
	rm.connections[url] = conn
	log.Printf("Connected to the external relay at: %s", url)

	return nil
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

	log.Printf("Subscribed to the events from relay %s", conn.URL)

	for event := range sub.Events {
		log.Printf("Recieved event %s from relay %s", event.ID[:8], conn.URL)
		// TODO - Event handling plz
	}
}

func (rm *RelayManager) Close() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for url, conn := range rm.connections {
		conn.Relay.Close()
		log.Printf("Closed the connection to the external relay at: %s", url)
	}
}
