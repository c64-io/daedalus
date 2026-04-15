package cliinteraction

import (
	"io"
	"os"
)

// styler emits ANSI SGR escapes for headers, labels, and prompts in
// the AI dialog. It auto-detects TTY support so that piping d7 into
// a file or buffer (including every Go test here) produces plain
// text, and it honors the NO_COLOR convention.
type styler struct {
	on bool
}

// newStyler enables color iff out is a character device and the
// NO_COLOR env var is unset.
func newStyler(out io.Writer) styler {
	if os.Getenv("NO_COLOR") != "" {
		return styler{on: false}
	}
	f, ok := out.(*os.File)
	if !ok {
		return styler{on: false}
	}
	info, err := f.Stat()
	if err != nil {
		return styler{on: false}
	}
	return styler{on: (info.Mode() & os.ModeCharDevice) != 0}
}

// wrap applies an SGR code to s when styling is enabled. Empty
// strings pass through untouched so formatting calls stay cheap.
func (s styler) wrap(code, text string) string {
	if !s.on || text == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (s styler) bold(t string) string        { return s.wrap("1", t) }
func (s styler) dim(t string) string         { return s.wrap("2", t) }
func (s styler) boldCyan(t string) string    { return s.wrap("1;36", t) }
func (s styler) boldMagenta(t string) string { return s.wrap("1;35", t) }
func (s styler) boldGreen(t string) string   { return s.wrap("1;32", t) }
func (s styler) yellow(t string) string      { return s.wrap("33", t) }
