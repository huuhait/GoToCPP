// Package fields has a field that is named like a header extension.
package fields

// Mixed has an exported field and an unexported one named h.
type Mixed struct {
	// H is exported.
	H int
	// h is not.
	h int
}
