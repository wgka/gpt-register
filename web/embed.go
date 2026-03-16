package web

import "embed"

// Dist stores the built Vue assets. Run `npm run build` in `frontend/` before building the Go binary.
//
//go:embed dist dist/*
var Dist embed.FS
