//go:build windows

package app

import (
	"unsafe"

	gioapp "gioui.org/app"
	"golang.org/x/sys/windows"
)

const (
	monitorDefaultToNearest = 2
	swpNoSize               = 0x0001
	swpNoZOrder             = 0x0004
	swpNoActivate           = 0x0010
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

type monitorInfo struct {
	Size    uint32
	Monitor winRect
	Work    winRect
	Flags   uint32
}

var (
	user32             = windows.NewLazySystemDLL("user32.dll")
	procGetWindowRect  = user32.NewProc("GetWindowRect")
	procMonitorForWnd  = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfo = user32.NewProc("GetMonitorInfoW")
	procSetWindowPos   = user32.NewProc("SetWindowPos")
	windowCentered     bool
)

// centerWindowOnFirstView centers only the initial native Windows window.
// Gio intentionally leaves placement to the OS, so this completes the desktop
// app behaviour without affecting macOS or Linux window managers.
func centerWindowOnFirstView(_ *gioapp.Window, event any) {
	if windowCentered {
		return
	}
	view, ok := event.(gioapp.Win32ViewEvent)
	if !ok || view.HWND == 0 {
		return
	}
	hwnd := uintptr(view.HWND)
	var windowRect winRect
	if ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&windowRect))); ret == 0 {
		return
	}
	monitor, _, _ := procMonitorForWnd.Call(hwnd, monitorDefaultToNearest)
	if monitor == 0 {
		return
	}
	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ret, _, _ := procGetMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ret == 0 {
		return
	}
	width := windowRect.Right - windowRect.Left
	height := windowRect.Bottom - windowRect.Top
	x := info.Work.Left + (info.Work.Right-info.Work.Left-width)/2
	y := info.Work.Top + (info.Work.Bottom-info.Work.Top-height)/2
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
	windowCentered = true
}
