package docs

import "embed"

// Schemas embeds the same schemas used by editors and documentation.
//
//go:embed video.schema.json template.schema.json
var Schemas embed.FS
