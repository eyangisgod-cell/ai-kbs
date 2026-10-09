package domain

// SAGEventEntityLink joins an event to an entity with a weight and a
// semantic role. Maps to the `sag_event_entity_links` table.
type SAGEventEntityLink struct {
	EventID  string
	EntityID string
	Weight   float32
	Role      string
}