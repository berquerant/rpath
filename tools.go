//go:build tools
// +build tools

package main

import (
	_ "github.com/joerdav/xc/cmd/xc"
	_ "golang.org/x/vuln/cmd/govulncheck"
)
