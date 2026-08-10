// Package prompt assembles generation requests from the official guides,
// plans their context budget, and audits and repairs model outputs.
package prompt

import (
	"errors"
	"fmt"
	"strings"
)

// Error is a stable, user-facing prompt-pipeline failure.
type Error struct {
	Code    string
	Message string
	Details any
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// Is reports whether err is a prompt Error, optionally with a specific code.
func Is(err error, code string) bool {
	var perr *Error
	return errors.As(err, &perr) && (code == "" || perr.Code == code)
}

func trimSpace(value string) string { return strings.TrimSpace(value) }

// formatInt renders 8000 as "8,000".
func formatInt(value int) string {
	digits := fmt.Sprintf("%d", value)
	var groups []string
	for len(digits) > 3 {
		groups = append([]string{digits[len(digits)-3:]}, groups...)
		digits = digits[:len(digits)-3]
	}
	return strings.Join(append([]string{digits}, groups...), ",")
}
