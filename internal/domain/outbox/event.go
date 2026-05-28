// Package outbox holds the transactional outbox aggregate for the identity
// service. Events are written inside the same pgx.Tx as the state change
// they describe. A separate drainer (not in walking skeleton) publishes
// them to Kafka.
package outbox

import (
	"time"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
)

// Event topic names emitted by this service. Versioned.
const (
	TopicUserCreated     = "identity.user.created.v1"
	TopicSessionCreated  = "identity.session.created.v1"
	TopicSessionRevoked  = "identity.session.revoked.v1"
)

// Event is one outbox row.
type Event struct {
	id            uuid.UUID
	tenantID      tenant.ID
	aggregateType string
	aggregateID   uuid.UUID
	eventType     string
	payload       []byte // canonical JSON
	createdAt     time.Time
}

// New constructs a new outbox event. payload must be canonical JSON.
func New(id uuid.UUID, t tenant.ID, aggregateType string, aggregateID uuid.UUID, eventType string, payload []byte, now time.Time) Event {
	return Event{
		id:            id,
		tenantID:      t,
		aggregateType: aggregateType,
		aggregateID:   aggregateID,
		eventType:     eventType,
		payload:       payload,
		createdAt:     now,
	}
}

func (e Event) ID() uuid.UUID          { return e.id }
func (e Event) TenantID() tenant.ID    { return e.tenantID }
func (e Event) AggregateType() string  { return e.aggregateType }
func (e Event) AggregateID() uuid.UUID { return e.aggregateID }
func (e Event) EventType() string      { return e.eventType }
func (e Event) Payload() []byte        { return e.payload }
func (e Event) CreatedAt() time.Time   { return e.createdAt }
