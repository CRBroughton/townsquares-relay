package manager

import (
	"github.com/nbd-wtf/go-nostr"
)

type RelayConnection struct {
	URL   string
	Relay *nostr.Relay
}

type RelayManager struct {
	connections map[string]*RelayConnection
}

func NewRelayManager() *RelayManager {
	return &RelayManager{
		connections: make(map[string]*RelayConnection),
	}
}
