package core

import "embed"

//go:embed all:frontend/dist
var FrontendAssets embed.FS

//go:embed frontend/public/logo.ico
var AppIcon []byte
