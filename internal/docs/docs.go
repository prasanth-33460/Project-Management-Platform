// Package docs embeds the OpenAPI spec and Swagger UI at compile time.
// Both files are baked into the binary — no runtime file I/O, no missing-file errors.
package docs

import _ "embed"

//go:embed openapi.json
var OpenAPIJSON []byte

//go:embed swagger.html
var SwaggerHTML []byte
