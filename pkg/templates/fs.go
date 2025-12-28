package templates

import "embed"

// Assets contains the embedded template files from the assets directory.
//
//go:embed assets/*
var Assets embed.FS
