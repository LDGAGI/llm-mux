package misc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/nghyane/llm-mux/internal/logging"
)

// Separator used to visually group related log lines.
var credentialSeparator = strings.Repeat("-", 67)

// LogSavingCredentials emits a consistent log message when persisting auth material.
func LogSavingCredentials(path string) {
	if path == "" {
		return
	}
	clean := strings.Replace(filepath.Clean(path), os.Getenv("HOME"), "~", 1)
	fmt.Printf("Saving credentials to to %s\n", clean)
}

// LogCredentialSeparator adds a visual separator to group auth/key processing logs.
func LogCredentialSeparator() {
	log.Debug(credentialSeparator)
}
