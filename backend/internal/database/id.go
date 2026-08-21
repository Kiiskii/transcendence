package database

import "github.com/google/uuid"

// NewID returns a primary key: a UUID v7, whose leading bits are a millisecond
// timestamp and a counter. That ordering is load-bearing - messages are listed
// with ORDER BY id and paginated with id > $2, and order_events uses id to
// break ties between two rows written in the same transaction. A v4 would make
// both arbitrary.
//
// Must, not an error return: NewV7 fails only when the system entropy source
// does, which the process cannot continue past. The panic surfaces as a logged
// 500 through Recoverer.
func NewID() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}
