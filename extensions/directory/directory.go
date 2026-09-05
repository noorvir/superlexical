// Package directory supplies the Directory record schema.
package directory

import _ "embed"

// Schema defines the required Directory fields and allows extra fields.
//
//go:embed schema.json
var Schema []byte
