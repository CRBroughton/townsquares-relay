package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
)

func main() {
	relay := khatru.NewRelay()

	relay.Info.Name = "Townsquares relay"
	relay.Info.PubKey = "PUB_KEY_HERE"
	relay.Info.Description = "A townsquares relay"

	// TODO - add actual storage
	store := make(map[string]*nostr.Event, 120)

	relay.StoreEvent = append(relay.StoreEvent, func(ctx context.Context, event *nostr.Event) error {
		store[event.ID] = event
		return nil
	})

	relay.QueryEvents = append(relay.QueryEvents, func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
		ch := make(chan *nostr.Event)
		go func() {
			for _, event := range store {
				if filter.Matches(event) {
					ch <- event
				}
			}
			close(ch)
		}()
		return ch, nil
	})

	mux := relay.Router()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/html")
	})

	fmt.Println("running on :3334")
	http.ListenAndServe(":3334", relay)
}
