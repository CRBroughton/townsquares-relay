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

func (rm *RelayManager) ConnectToRelay(ctx context.Context, url string) error {
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
