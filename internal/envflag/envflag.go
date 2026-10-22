// Package envflag reads the on/off environment variables of gocpp that more than one package reads,
// so that they mean the same everywhere.
package envflag

import "strings"

// OfflineSpellings is how the documentation says GOCPP_OFFLINE is read.
const OfflineSpellings = "1, true, yes or on turn it on; empty, 0, false, no or off turn it off (any case); any other value is on too, so that a typo never uses the network"

// Offline reports whether the value v of GOCPP_OFFLINE means "never use the network": 1, true, yes
// and on (in any case, spaces around them ignored), and every value that is not one of the off
// spellings: empty, 0, false, no and off. A word that is neither (a typo) is on: the variable is a
// safety switch, and a wrong spelling must not be the one that goes online.
func Offline(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "no", "off":
		return false
	}
	return true
}
