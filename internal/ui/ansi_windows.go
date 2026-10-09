//go:build windows

package ui

import (
	"os"
	"syscall"
	"unsafe"
)

const enableVirtualTerminalProcessing = 0x0004

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode = kernel32.NewProc("GetConsoleMode")
	setConsoleMode = kernel32.NewProc("SetConsoleMode")
)

// enableANSI turns on ANSI escape processing for a Windows console.
//
// Windows Terminal handles escapes already, but the classic console host
// prints them as raw text unless this flag is set. It returns false when
// the console refuses, so the caller can fall back to plain output rather
// than print garbage.
func enableANSI(f *os.File) bool {
	handle := f.Fd()

	var mode uint32
	if r, _, _ := getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode))); r == 0 {
		// Not a console (a pipe, or a Cygwin/MSYS terminal that handles
		// escapes itself). Leave the decision to the caller's tty check.
		return true
	}
	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}
	r, _, _ := setConsoleMode.Call(handle, uintptr(mode|enableVirtualTerminalProcessing))
	return r != 0
}
