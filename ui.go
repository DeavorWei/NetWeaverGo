package core

import "embed"

//go:embed all:frontend/dist
var FrontendAssets embed.FS
