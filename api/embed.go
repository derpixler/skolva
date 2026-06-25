// Package api embeds the OpenAPI specification so the binary can serve it
// without relying on files on disk at runtime.
package api

import _ "embed"

//go:embed openapi.yaml
var Spec []byte
