// Package macro holds the hidden call-site type the compiler passes to every macro.
package macro

// Site describes where a macro was used. One static value per use (zero runtime cost).
type Site struct {
	File string
	Line int
	Func string
	Args []string // source text of each argument at the use site
}
