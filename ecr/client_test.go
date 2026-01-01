package ecr

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	config := &Config{
		POSHost: "localhost",
		POSPort: 8080,
	}

	client := NewClient(config)

	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if client.config != config {
		t.Error("Config not set correctly")
	}

	if client.connected {
		t.Error("Client should not be connected initially")
	}

	if client.sequence != 0 {
		t.Errorf("Initial sequence: got %d, want 0", client.sequence)
	}
}

func TestNewClientWithNilConfig(t *testing.T) {
	client := NewClient(nil)

	if client == nil {
		t.Fatal("NewClient with nil config should use default")
	}

	if client.config == nil {
		t.Fatal("Client config should not be nil")
	}
}

func TestClientSequenceIncrement(t *testing.T) {
	client := NewClient(&Config{
		POSHost: "localhost",
		POSPort: 8080,
	})

	// Test that sequence increments (using internal method)
	client.seqMutex.Lock()
	seq1 := client.sequence
	client.sequence++
	seq2 := client.sequence
	client.sequence++
	seq3 := client.sequence
	client.seqMutex.Unlock()

	if seq1 == seq2 || seq2 == seq3 {
		t.Error("Sequence numbers should increment")
	}

	if seq2 != seq1+1 {
		t.Errorf("Sequence should increment by 1")
	}
}

func TestClientSequenceConcurrency(t *testing.T) {
	client := NewClient(&Config{
		POSHost: "localhost",
		POSPort: 8080,
	})

	sequences := make(chan uint16, 50)
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				client.seqMutex.Lock()
				client.sequence++
				seq := client.sequence
				client.seqMutex.Unlock()
				sequences <- seq
			}
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	close(sequences)

	seen := make(map[uint16]bool)
	count := 0
	for seq := range sequences {
		if seen[seq] {
			t.Errorf("Duplicate sequence: %d", seq)
		}
		seen[seq] = true
		count++
	}

	if count != 50 {
		t.Errorf("Expected 50 sequences, got %d", count)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if config.POSHost == "" {
		t.Error("POSHost should be set")
	}

	if config.POSPort == 0 {
		t.Error("POSPort should be set")
	}

	if config.ConnectTimeout == 0 {
		t.Error("ConnectTimeout should be set")
	}
}

func TestClientNotConnected(t *testing.T) {
	client := NewClient(&Config{
		POSHost: "localhost",
		POSPort: 9999,
	})

	if client.IsConnected() {
		t.Error("Client should not be connected initially")
	}
}

func TestConfigValues(t *testing.T) {
	config := &Config{
		POSHost:        "192.168.1.1",
		POSPort:        8080,
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxRetries:     3,
		RetryDelay:     1 * time.Second,
	}

	client := NewClient(config)

	if client.config.POSHost != "192.168.1.1" {
		t.Error("POSHost not set correctly")
	}

	if client.config.POSPort != 8080 {
		t.Error("POSPort not set correctly")
	}

	if client.config.MaxRetries != 3 {
		t.Error("MaxRetries not set correctly")
	}
}

func TestClientIsConnected(t *testing.T) {
	client := NewClient(&Config{
		POSHost: "localhost",
		POSPort: 8080,
	})

	if client.IsConnected() {
		t.Error("Client should not be connected initially")
	}

	client.connMutex.Lock()
	client.connected = true
	client.connMutex.Unlock()

	if !client.IsConnected() {
		t.Error("Client should report as connected")
	}
}

func TestClientDisconnect(t *testing.T) {
	client := NewClient(&Config{
		POSHost: "localhost",
		POSPort: 8080,
	})

	client.connMutex.Lock()
	client.connected = true
	client.connMutex.Unlock()

	err := client.Disconnect()
	if err != nil {
		t.Errorf("Disconnect returned error: %v", err)
	}

	if client.IsConnected() {
		t.Error("Client should be disconnected")
	}
}
