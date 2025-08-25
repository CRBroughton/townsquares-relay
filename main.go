package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fiatjaf/eventstore/badger"
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
	DBPath      string   `json:"db_path"`
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
	configFile := "config.json"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}
	config, err := loadConfig(configFile)

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
		go func(relayURL string) {
			for {
				err := relayManager.Connect(ctx, relayURL)
				if err != nil {
					time.Sleep(10 * time.Second)
					continue
				}
				break
			}
		}(relayURL)
	}

	defer relayManager.Close()

	relayManager.StartSubscriptions(ctx)

	dbPath := config.DBPath
	if dbPath == "" {
		dbPath = "db"
	}

	db := &badger.BadgerBackend{
		Path: dbPath,
	}
	if err := db.Init(); err != nil {
		log.Fatalf("Failed to initialize BadgerDB: %v", err)
	}
	defer db.Close()

	relay.StoreEvent = append(relay.StoreEvent, func(ctx context.Context, event *nostr.Event) error {
		if err := db.SaveEvent(ctx, event); err != nil {
			return err
		}

		clientIP := khatru.GetIP(ctx)
		log.Printf("Received event %s from relay %s", event.ID[:8], clientIP)
		relayManager.Broadcast(ctx, event)
		return nil
	})

	relay.QueryEvents = append(relay.QueryEvents, func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
		ch := make(chan *nostr.Event)
		go func() {
			defer close(ch)

			// Query local BadgerDB storage
			localCh, err := db.QueryEvents(ctx, filter)
			if err != nil {
				return
			}

			seenEvents := make(map[string]bool)

			// Send events from local storage
			for event := range localCh {
				seenEvents[event.ID] = true
				select {
				case ch <- event:
				case <-ctx.Done():
					return
				}
			}

			// Query events from connected relays
			for _, event := range relayManager.GetAllEvents() {
				if filter.Matches(event) {
					if !seenEvents[event.ID] {
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
