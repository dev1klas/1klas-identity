package postgres

import "time"

// newTimes returns two pre-allocated time pointers for Scan targets.
// Tiny helper to keep call sites tidy.
func newTimes() (*time.Time, *time.Time) {
	var a, b time.Time
	return &a, &b
}
