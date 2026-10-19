package diag

import (
	"io"
	"os"
	"runtime"
	"strings"
)

// The styles of a Printer with Color: ANSI escape sequences, no library.
const (
	styleBold     = "\x1b[1m"
	styleErrorMsg = "\x1b[1;31m" // bold red
	styleWarning  = "\x1b[1;33m" // bold yellow
	styleNote     = "\x1b[35m"   // magenta
	styleCaret    = "\x1b[1;32m" // bold green
	styleReset    = "\x1b[0m"
)

// paint returns text in the style, if the Printer has Color. Each line of
// text gets the style on its own (and a reset before its newline), so that
// a pager or a terminal that cuts the output at a line end can't leak a
// color into the next line. An empty style or text is returned as it is.
func (s *printState) paint(style, text string) string {
	if !s.Color || style == "" || text == "" {
		return text
	}
	if !strings.Contains(text, "\n") {
		return style + text + styleReset
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = style + l + styleReset
		}
	}
	return strings.Join(lines, "\n")
}

// ColorEnabled reports whether a program that writes to w should color its
// output, for the color setting mode ("auto", "always" or "never"; "" is
// "auto") and the environment getenv. The first rule that applies wins:
//
//  1. mode "always" or "never" (the -color flag);
//  2. GOCPP_COLOR "always" or "never" ("auto" or empty: go on);
//  3. NO_COLOR set to a non-empty value: no color (no-color.org);
//  4. CLICOLOR_FORCE set to a value other than "" and "0": color
//     (bixense.com/clicolors);
//  5. TERM is "dumb": no color;
//  6. color if w is a terminal.
//
// A bad mode or GOCPP_COLOR counts as "auto"; ParseColorMode checks a flag
// value. On Windows a terminal counts only with TERM or WT_SESSION (Windows
// Terminal) set, because older consoles don't read ANSI sequences.
func ColorEnabled(mode string, w io.Writer, getenv func(string) string) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	}
	switch getenv("GOCPP_COLOR") {
	case "always":
		return true
	case "never":
		return false
	}
	if getenv("NO_COLOR") != "" {
		return false
	}
	if v := getenv("CLICOLOR_FORCE"); v != "" && v != "0" {
		return true
	}
	term := getenv("TERM")
	if term == "dumb" {
		return false
	}
	if runtime.GOOS == "windows" && term == "" && getenv("WT_SESSION") == "" {
		return false
	}
	return IsTerminal(w)
}

// ParseColorMode checks the value of a -color flag.
func ParseColorMode(s string) (string, bool) {
	switch s {
	case "", "auto":
		return "auto", true
	case "always", "never":
		return s, true
	}
	return "", false
}

// IsTerminal reports whether w is a terminal: a file that is a character
// device (a terminal, but also /dev/null, which is no problem: nobody reads
// what goes there).
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0 && !isNull(fi)
}

// isNull reports whether fi is the null device (a character device too).
func isNull(fi os.FileInfo) bool {
	null, err := os.Stat(os.DevNull)
	return err == nil && os.SameFile(fi, null)
}
