package natsbus

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/flagmate/suricata-ctf/backend/internal/models"
)

type NATSBus struct {
	nc      *nats.Conn
	subject string
	mu      sync.RWMutex
	clients map[string]*Client
}

type Client struct {
	ID   string
	Ch   chan models.Flow
	Done chan struct{}
}

func New(url string, subject string) (*NATSBus, error) {
	nc, err := nats.Connect(url,
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, logError("failed to connect to NATS: %v", err)
	}

	bus := &NATSBus{
		nc:      nc,
		subject: subject,
		clients: make(map[string]*Client),
	}

	go bus.subscribe()
	log.Printf("NATS bus connected to %s, subject: %s", url, subject)
	return bus, nil
}

func (nb *NATSBus) Publish(flow models.Flow) error {
	data, err := json.Marshal(flow)
	if err != nil {
		return err
	}

	return nb.nc.Publish(nb.subject, data)
}

func (nb *NATSBus) AddClient(id string) *Client {
	nb.mu.Lock()
	defer nb.mu.Unlock()

	client := &Client{
		ID:   id,
		Ch:   make(chan models.Flow, 100),
		Done: make(chan struct{}),
	}

	nb.clients[id] = client
	return client
}

func (nb *NATSBus) RemoveClient(id string) {
	nb.mu.Lock()
	defer nb.mu.Unlock()

	if client, exists := nb.clients[id]; exists {
		close(client.Done)
		delete(nb.clients, id)
	}
}

func (nb *NATSBus) ClientCount() int {
	nb.mu.RLock()
	defer nb.mu.RUnlock()
	return len(nb.clients)
}

func (nb *NATSBus) subscribe() {
	_, err := nb.nc.Subscribe(nb.subject, func(msg *nats.Msg) {
		var flow models.Flow
		if err := json.Unmarshal(msg.Data, &flow); err != nil {
			log.Printf("Failed to unmarshal flow from NATS: %v", err)
			return
		}

		nb.mu.RLock()
		for _, client := range nb.clients {
			select {
			case client.Ch <- flow:
			default:
			}
		}
		nb.mu.RUnlock()
	})

	if err != nil {
		log.Printf("Failed to subscribe to NATS subject %s: %v", nb.subject, err)
	}
}

func (nb *NATSBus) Close() {
	if nb.nc != nil {
		nb.nc.Close()
	}
}

func logError(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	log.Printf("NATSBus: %v", err)
	return err
}
