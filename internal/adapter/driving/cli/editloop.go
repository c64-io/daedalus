package cli

import (
	"fmt"

	"github.com/c64-io/daedalus/internal/core/port"
)

// editLoop opens the editor on initialContent, then calls parse on
// the result. If parse returns an error, the error is prepended as a
// comment and the editor is re-opened. On success, returns the final
// edited content.
func editLoop(editor port.Editor, initialContent string, parse func(string) error) (string, error) {
	content := initialContent
	for {
		edited, err := editor.Edit(content)
		if err != nil {
			return "", err
		}

		if parseErr := parse(edited); parseErr != nil {
			// Prepend error as a comment and re-open.
			content = fmt.Sprintf("# ERROR: %s\n%s", parseErr.Error(), edited)
			continue
		}

		return edited, nil
	}
}
