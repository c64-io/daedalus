package driven

// Editor is a driven port abstracting the user's text editor. The
// service builds the content string; the adapter handles temp files
// and launching the editor process.
type Editor interface {
	// Edit opens the user's editor on the given content and returns the
	// edited content when the editor exits. Implementations should use
	// $EDITOR (falling back to vi) and a temp file with a .md extension
	// for syntax highlighting.
	Edit(content string) (string, error)
}
