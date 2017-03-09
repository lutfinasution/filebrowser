package main

import (
	"syscall"
	//"unsafe"

	"github.com/lxn/win"
)

var (
	// Library
	libuser32 uintptr

	// Functions
	getForegroundWindow uintptr
)

func init() {
	//is64bit := unsafe.Sizeof(uintptr(0)) == 8

	// Library
	libuser32 = win.MustLoadLibrary("user32.dll")

	// Functions
	getForegroundWindow = win.MustGetProcAddress(libuser32, "GetForegroundWindow")
}

func GetForegroundWindow() win.HWND {
	ret, _, _ := syscall.Syscall(getForegroundWindow, 0, 0, 0, 0)

	return win.HWND(ret)
}
