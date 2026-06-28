// Package api embeds the OpenAPI specification and Redoc UI assets so the
// binary can serve them without relying on files on disk at runtime.
package api

import _ "embed"

//go:embed openapi.yaml
var Spec []byte

//go:embed redoc.html
var RedocHTML []byte

//go:embed redoc.js
var RedocJS []byte
