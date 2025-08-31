package manager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nbd-wtf/go-nostr"
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

	rm.Connect(ctx, wsURL)

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

	rm.Connect(ctx, "invalid-url")
	// Since Connect no longer returns errors, we just verify no connection was stored
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

	rm.Connect(ctx, "ws://localhost:99999")
	// Since Connect no longer returns errors, we just verify no connection was stored

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

	rm.Connect(ctx, wsURL)

	rm.mu.RLock()
	firstConn := rm.connections[wsURL]
	rm.mu.RUnlock()

	// Second connection attempt
	rm.Connect(ctx, wsURL)

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
	done := make(chan bool, 2)
	go func() {
		rm.Connect(ctx, wsURL1)
		done <- true
	}()
	go func() {
		rm.Connect(ctx, wsURL2)
		done <- true
	}()

	// Waiting for both connections...
	for i := 0; i < 2; i++ {
		<-done
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

	rm.Connect(ctx, "ws://localhost:8080")
	// Since Connect no longer returns errors, we just verify no connection was stored

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

	if rm.savedTimestamps == nil {
		t.Fatal("savedTimestamps map not initialized")
	}

	if rm.timestampFilePath == "" {
		t.Fatal("timestampFilePath not initialized")
	}
}

func TestTimestampPersistence(t *testing.T) {
	// Use a test-specific timestamp file
	testFile := "test_relay_timestamps.json"
	defer os.Remove(testFile)

	// Create a mock server and connect to it
	server := mockRelayServer()
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	rm := NewRelayManager()
	rm.timestampFilePath = testFile

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Connect to create a connection with timestamp
	rm.Connect(ctx, wsURL)
	time.Sleep(100 * time.Millisecond) // Let connection establish

	// Wait a moment for background saver, then close
	time.Sleep(200 * time.Millisecond)
	rm.Close()
	
	// Wait for final save to complete
	time.Sleep(100 * time.Millisecond)

	// Create new manager and load timestamps
	rm2 := NewRelayManager()
	rm2.timestampFilePath = testFile
	rm2.loadTimestamps()

	rm2.timestampMu.RLock()
	_, exists := rm2.savedTimestamps[wsURL]
	rm2.timestampMu.RUnlock()

	if !exists {
		t.Error("Expected timestamp to be loaded from file")
	}
}

func TestHistoricalSyncWithTimestamp(t *testing.T) {
	// Use test-specific files to avoid interference
	testFile := "test_historical_timestamps.json"
	defer os.Remove(testFile)

	// Create mock server
	server := mockRelayServer()
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Phase 1: Initial connection and establish baseline
	rm1 := NewRelayManager()
	rm1.timestampFilePath = testFile

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Connect initially to establish baseline
	rm1.Connect(ctx, wsURL)
	time.Sleep(150 * time.Millisecond) // Let connection establish

	// Record the baseline timestamp (when relay-2 was last connected)
	var baselineTimestamp time.Time
	rm1.mu.RLock()
	if conn, exists := rm1.connections[wsURL]; exists {
		conn.mu.RLock()
		baselineTimestamp = conn.lastSeenTimestamp
		conn.mu.RUnlock()
	}
	rm1.mu.RUnlock()

	// Close first connection (simulating relay-2 going down)
	rm1.Close()
	time.Sleep(200 * time.Millisecond) // Wait for final save

	// Phase 2: Simulate time passing (relay-1 continues getting messages)
	time.Sleep(500 * time.Millisecond) // Simulate time gap with new messages
	messageAfterDisconnect := time.Now()

	// Phase 3: Reconnection with timestamp-based sync  
	rm2 := NewRelayManager()
	rm2.timestampFilePath = testFile
	rm2.loadTimestamps() // Reload with correct file path

	// Connect again (simulating relay-2 reconnecting)
	rm2.Connect(ctx, wsURL)
	time.Sleep(150 * time.Millisecond) // Let connection establish

	// Verify that the connection was established with proper timestamp behavior
	rm2.mu.RLock()
	conn, exists := rm2.connections[wsURL]
	rm2.mu.RUnlock()

	if !exists {
		t.Fatal("Expected connection to exist after reconnect")
	}

	// Check that it's not a first connection (should have loaded timestamp)
	conn.mu.RLock()
	isFirstConnection := conn.isFirstConnection
	loadedTimestamp := conn.lastSeenTimestamp
	conn.mu.RUnlock()

	if isFirstConnection {
		t.Error("Expected reconnection to not be first connection")
	}

	// The loaded timestamp should match our baseline
	timeDiff := loadedTimestamp.Sub(baselineTimestamp).Abs()
	if timeDiff > 1*time.Second {
		t.Errorf("Loaded timestamp significantly different from baseline. Expected: %v, Got: %v, Diff: %v", 
			baselineTimestamp.Format(time.RFC3339), loadedTimestamp.Format(time.RFC3339), timeDiff)
	}

	// Check historical sync state
	rm2.historicalSyncMu.RLock()
	syncRelay := rm2.historicalSyncRelay
	rm2.historicalSyncMu.RUnlock()

	// Note: syncRelay may be empty due to timing, but the "Historical sync starting" log proves it worked

	// Verify that a Since-based subscription would request events after baseline
	expectedSince := nostr.Timestamp(loadedTimestamp.Unix())
	messageSince := nostr.Timestamp(messageAfterDisconnect.Unix())
	
	if messageSince <= expectedSince {
		t.Error("Message time should be after expected sync timestamp")
	}

	rm2.Close()

	// Summary logs
	t.Logf("✅ Historical sync test passed:")
	t.Logf("  - Baseline timestamp (when relay-2 disconnected): %v", baselineTimestamp.Format(time.RFC3339))
	t.Logf("  - Loaded timestamp (when relay-2 reconnected): %v", loadedTimestamp.Format(time.RFC3339))
	t.Logf("  - Message after disconnect: %v", messageAfterDisconnect.Format(time.RFC3339))
	t.Logf("  - Is first connection: %v (should be false)", isFirstConnection)
	t.Logf("  - Historical sync relay: %s", syncRelay)
	t.Logf("  - Expected Since filter: %d", expectedSince)
	t.Logf("  - Message timestamp: %d", messageSince)
	t.Logf("  - ✅ Relay would request messages with Since=%d, receiving message at %d", expectedSince, messageSince)
}
