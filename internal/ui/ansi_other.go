//go:build !windows

package ui

import "os"

// enableANSI is only needed on Windows; elsewhere a terminal already
// understands escapes.
func enableANSI(*os.File) bool { return true }
