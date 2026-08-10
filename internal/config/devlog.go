package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DevMode enables the JSONL generation log via VWRITER_DEV_MODE.
var DevMode = func() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("VWRITER_DEV_MODE")))
	return value != "" && value != "0" && value != "false" && value != "off" && value != "no"
}()

// DevLogPath locates generations.jsonl next to the executable.
func DevLogPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "generations.jsonl"
	}
	return filepath.Join(filepath.Dir(exe), "generations.jsonl")
}

var devLogMu sync.Mutex

// LogEvent appends one JSON record to the developer log. Logging must never
// break a generation request, so write errors are swallowed.
func LogEvent(event string, fields map[string]any) {
	if !DevMode {
		return
	}
	record := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"event":     event,
	}
	for key, value := range fields {
		record[key] = value
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return
	}
	devLogMu.Lock()
	defer devLogMu.Unlock()
	file, err := os.OpenFile(DevLogPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	file.Write(append(raw, '\n'))
}
