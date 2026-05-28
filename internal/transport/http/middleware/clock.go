package middleware

import "time"

// nowFn is a package-level seam so tests can freeze time without rewiring
// every middleware. Defaults to time.Now().UTC().
var nowFn = func() time.Time { return time.Now().UTC() }
