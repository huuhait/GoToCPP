// Package macro is the hidden gp_macro package: what a macro knows about its use site.
package macro

type Site struct {
	File string
	Line int
	Func string
}
