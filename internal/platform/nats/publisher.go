package nats

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

// Publisher wraps a NATS connection for publishing JSON-encoded events.
type Publisher struct {
	conn *nats.Conn
}

// NewPublisher creates a Publisher using the provided NATS connection.
func NewPublisher(conn *nats.Conn) *Publisher {
	return &Publisher{conn: conn}
}

// Publish marshals the event to JSON and publishes it to the given subject.
func (p *Publisher) Publish(subject string, event any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	if err := p.conn.Publish(subject, data); err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}

	return nil
}

// Close drains and closes the underlying NATS connection.
func (p *Publisher) Close() {
	p.conn.Close()
}
