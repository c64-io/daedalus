package editorexec

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/c64-io/daedalus/internal/core/port"
)

// Compile-time assertion that Editor satisfies the driven port.
var _ port.Editor = (*Editor)(nil)

// Editor launches the user's $EDITOR on a temporary file and returns
// the edited content. It falls back to "vi" when $EDITOR is unset.
type Editor struct{}

// New returns an Editor adapter.
func New() *Editor { return &Editor{} }

// Edit writes content to a temp file, opens $EDITOR, waits for it to
// exit, reads back the file, and cleans up.
func (e *Editor) Edit(content string) (string, error) {
	f, err := os.CreateTemp("", "d7-edit-*.md")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := f.Name()
	defer os.Remove(tmpPath)

	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("run editor: %w", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("read edited file: %w", err)
	}

	return string(data), nil
}
