// Package apispec embeds the OpenAPI 3.0 document at compile time.
package apispec

import _ "embed"

// Spec is the identity service's OpenAPI 3.0 document.
//
//go:embed openapi.json
var Spec []byte
