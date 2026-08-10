//go:build !windows

package app

import gioapp "gioui.org/app"

// centerWindowOnFirstView is a platform hook. Window placement is delegated
// to the compositor on platforms other than Windows.
func centerWindowOnFirstView(_ *gioapp.Window, _ any) {}
