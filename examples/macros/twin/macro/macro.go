// Package macro stands for gocpp's hidden gocpp/macro package (imported as
// gp_macro): what a macro knows about the place where it is used.
package macro

// Site is one use of a macro. The compiler makes one static Site per use.
type Site struct {
	File  string   // #file()
	Line  int      // #line()
	Func  string   // #func()
	Args  []string // #text(arg): the source text of each argument
	Outer *Site    // the use of the macro whose body holds this use, or nil
}

// Outermost returns the site in the user's code: #file(), #line() and
// #func() describe it, while #text() reads the innermost site.
func (s *Site) Outermost() *Site {
	for s.Outer != nil {
		s = s.Outer
	}
	return s
}
