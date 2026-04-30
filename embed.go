package main

import "embed"

//go:embed web/index.html
var WebFS embed.FS
