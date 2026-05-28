// Package clock exposes a Clock abstraction so use cases can be
// unit-tested with frozen time.
package clock

import "time"

// Clock returns the current time. The Real implementation defers to
// time.Now(). Tests inject a fake.
type Clock interface {
	Now() time.Time
}

// Real is a Clock backed by time.Now().
type Real struct{}

// Now returns time.Now().UTC().
func (Real) Now() time.Time { return time.Now().UTC() }
