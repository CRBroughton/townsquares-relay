package manager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// This is a basic mock nostr relay server for testing
func mockRelayServer() *httptest.Server {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// A small mock message we sent to the connected websocket server
		infoMsg := []any{"NOTICE", "relay connected"}
		conn.WriteJSON(infoMsg)
		// Keep connection alive for a short time to ensure the message is sent
		time.Sleep(100 * time.Millisecond)
	}))

	return server
}

func TestCanSuccessfullyConnectToRelay(t *testing.T) {
	server := mockRelayServer()
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	rm := NewRelayManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := rm.ConnectToRelay(ctx, wsURL)
	if err != nil {
		t.Fatalf("Expected successful connection, got error: %v", err)
	}

	// Here we verify our mock connection was stored with our relay
	rm.mu.RLock()
	conn, exists := rm.connections[wsURL]
	rm.mu.RUnlock()

	if !exists {
		t.Fatal("Connection not found in manager")
	}

	if conn.URL != wsURL {
		t.Errorf("Expected URL %s, got %s", wsURL, conn.URL)
	}

	if !conn.active {
		t.Error("Expected connection to be active")
	}

	if conn.Relay == nil {
		t.Error("Expected relay to be set")
	}
}

func TestVerifyInvalidURLSArentStored(t *testing.T) {
	rm := NewRelayManager()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := rm.ConnectToRelay(ctx, "invalid-url")
	if err == nil {
		t.Fatal("Expected error for invalid URL, got nil")
	}
	rm.mu.RLock()
	count := len(rm.connections)
	rm.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 connections, got %d", count)
	}
}

func TestVerifyUnreachableURLSArentStored(t *testing.T) {
	rm := NewRelayManager()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := rm.ConnectToRelay(ctx, "ws://localhost:99999")
	if err == nil {
		t.Fatal("Expected error for unreachable URL, got nil")
	}

	rm.mu.RLock()
	count := len(rm.connections)
	rm.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 connections, got %d", count)
	}
}

func TestVerifyDuplicateConnectionsArentStored(t *testing.T) {
	server := mockRelayServer()
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	rm := NewRelayManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := rm.ConnectToRelay(ctx, wsURL)
	if err != nil {
		t.Fatalf("First connection failed: %v", err)
	}

	rm.mu.RLock()
	firstConn := rm.connections[wsURL]
	rm.mu.RUnlock()

	// Second connection attempt
	err = rm.ConnectToRelay(ctx, wsURL)
	if err != nil {
		t.Fatalf("Second connection failed: %v", err)
	}

	// Here we're verifying only one instance of the connection has been stored
	rm.mu.RLock()
	count := len(rm.connections)
	secondConn := rm.connections[wsURL]
	rm.mu.RUnlock()

	if count != 1 {
		t.Errorf("Expected 1 connection, got %d", count)
	}

	if firstConn != secondConn {
		t.Error("Expected same connection instance for duplicate connection attempt")
	}
}

func TestCanConnectToMultipleRelays(t *testing.T) {
	// Create multiple mock relay servers
	server1 := mockRelayServer()
	defer server1.Close()
	server2 := mockRelayServer()
	defer server2.Close()

	wsURL1 := "ws" + strings.TrimPrefix(server1.URL, "http")
	wsURL2 := "ws" + strings.TrimPrefix(server2.URL, "http")

	rm := NewRelayManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Here we're connecting to both relays concurrently
	errChan := make(chan error, 2)
	go func() {
		errChan <- rm.ConnectToRelay(ctx, wsURL1)
	}()
	go func() {
		errChan <- rm.ConnectToRelay(ctx, wsURL2)
	}()

	// Waiting for both connections...
	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil {
			t.Fatalf("Concurrent connection failed: %v", err)
		}
	}

	rm.mu.RLock()
	count := len(rm.connections)
	conn1, exists1 := rm.connections[wsURL1]
	conn2, exists2 := rm.connections[wsURL2]
	rm.mu.RUnlock()

	if count != 2 {
		t.Errorf("Expected 2 connections, got %d", count)
	}

	if !exists1 || !exists2 {
		t.Error("One or both connections missing")
	}

	if !conn1.active || !conn2.active {
		t.Error("One or both connections not active")
	}
}

func TestVerifyCancelledContextsArentStored(t *testing.T) {
	rm := NewRelayManager()

	// Create context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rm.ConnectToRelay(ctx, "ws://localhost:8080")
	if err == nil {
		t.Fatal("Expected error for cancelled context, got nil")
	}

	rm.mu.RLock()
	count := len(rm.connections)
	rm.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 connections, got %d", count)
	}
}

func TestNewRelayManager(t *testing.T) {
	rm := NewRelayManager()

	if rm == nil {
		t.Fatal("NewRelayManager returned nil")
	}

	if rm.connections == nil {
		t.Fatal("connections map not initialized")
	}

	if len(rm.connections) != 0 {
		t.Errorf("Expected empty connections map, got %d entries", len(rm.connections))
	}
}
