package http

import "embed"

// staticFiles stores UI assets directly in the binary for immutable deployments.
//
//go:embed static
var staticFiles embed.FS
