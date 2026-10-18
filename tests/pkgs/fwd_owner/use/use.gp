package use

import "example.com/fwdowner/outer"

type Getter interface{ Get() int }
type Adder interface{ Add(int) }

// Convert is the only place that converts outer.Wrapper to interfaces: the descriptor and the
// forwarders of the struct are written by package outer, which does not do that itself.
func Convert(w *outer.Wrapper) (Getter, Adder, any) {
	return w, w, *w
}
