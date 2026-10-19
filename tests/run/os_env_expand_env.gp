// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	os.Setenv("GPOS_X", "xval")
	os.Setenv("GPOS_Y", "yval")
	os.Setenv("GPOS_EMPTY", "")
	os.Unsetenv("GPOS_NONE")
	for _, s := range []string{
		"", "plain", "$GPOS_X", "${GPOS_X}", "$GPOS_X-$GPOS_Y", "${GPOS_X}_suffix", "$GPOS_X_suffix",
		"$GPOS_EMPTY|", "$GPOS_NONE|", "${GPOS_NONE}|", "$$", "$$GPOS_X", "$", "$ ", "x$", "${", "${}", "${GPOS_X",
		"$1", "$12", "$*", "$@", "$#", "$?", "$!", "$-", "$0", "${1}", "${*}", "a${GPOS_X}b${GPOS_Y}c", "$(GPOS_X)", "${GPOS_X}}", "$.", "$é", "\\$GPOS_X",
	} {
		fmt.Printf("%q -> %q\n", s, os.ExpandEnv(s))
	}
}

// Output:
// "" -> ""
// "plain" -> "plain"
// "$GPOS_X" -> "xval"
// "${GPOS_X}" -> "xval"
// "$GPOS_X-$GPOS_Y" -> "xval-yval"
// "${GPOS_X}_suffix" -> "xval_suffix"
// "$GPOS_X_suffix" -> ""
// "$GPOS_EMPTY|" -> "|"
// "$GPOS_NONE|" -> "|"
// "${GPOS_NONE}|" -> "|"
// "$$" -> ""
// "$$GPOS_X" -> "GPOS_X"
// "$" -> "$"
// "$ " -> "$ "
// "x$" -> "x$"
// "${" -> ""
// "${}" -> ""
// "${GPOS_X" -> "GPOS_X"
// "$1" -> ""
// "$12" -> "2"
// "$*" -> ""
// "$@" -> ""
// "$#" -> ""
// "$?" -> ""
// "$!" -> ""
// "$-" -> ""
// "$0" -> ""
// "${1}" -> ""
// "${*}" -> ""
// "a${GPOS_X}b${GPOS_Y}c" -> "axvalbyvalc"
// "$(GPOS_X)" -> "$(GPOS_X)"
// "${GPOS_X}}" -> "xval}"
// "$." -> "$."
// "$é" -> "$é"
// "\\$GPOS_X" -> "\\xval"
