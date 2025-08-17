package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
	"github.crom/crbroughton/townsquares-relay/manager"
)

type Config struct {
	Port        string   `json:"port"`
	Name        string   `json:"name"`
	PubKey      string   `json:"pubkey"`
	Description string   `json:"description"`
	Relays      []string `json:"relays"`
}

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func main() {
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	relay := khatru.NewRelay()
	relay.Info.Name = config.Name
	relay.Info.PubKey = config.PubKey
	relay.Info.Description = config.Description

	ctx := context.Background()
	relayManager := manager.NewRelayManager()
	for _, relayURL := range config.Relays {
		if err := relayManager.Connect(ctx, relayURL); err != nil {
			log.Printf("⚠️ Failed to connect to relay %s: %v", relayURL, err)
		}
	}
	defer relayManager.Close()

	// TODO - add actual storage
	store := make(map[string]*nostr.Event, 120)

	relay.StoreEvent = append(relay.StoreEvent, func(ctx context.Context, event *nostr.Event) error {
		store[event.ID] = event

		clientIP := khatru.GetIP(ctx)
		log.Printf("Received event %s from relay %s", event.ID[:8], clientIP)
		relayManager.Broadcast(ctx, event)
		return nil
	})

	relay.QueryEvents = append(relay.QueryEvents, func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
		ch := make(chan *nostr.Event)
		go func() {
			for _, event := range store {
				if filter.Matches(event) {
					select {
					case ch <- event:
					case <-ctx.Done():
						return
					}
				}
			}

			for _, event := range relayManager.GetAllEvents() {
				if filter.Matches(event) {
					if _, exists := store[event.ID]; !exists {
						select {
						case ch <- event:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}()
		return ch, nil
	})

	relay.OnConnect = append(relay.OnConnect, func(ctx context.Context) {
		clientIP := khatru.GetIP(ctx)
		log.Printf("New connection from %s", clientIP)
	})
	relay.OnDisconnect = append(relay.OnDisconnect, func(ctx context.Context) {
		clientIP := khatru.GetIP(ctx)
		log.Printf("Connection closed from %s", clientIP)
	})

	mux := relay.Router()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/html")
	})

	fmt.Printf("running on %s\n", config.Port)
	log.Fatal(http.ListenAndServe(config.Port, relay))
}
