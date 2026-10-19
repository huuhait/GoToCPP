package doc

import (
	"strings"

	godoc "go/doc"
)

// examples lists the examples of the _test.gp files (Options.Examples; go doc
// has nothing like it): all of them for the package (all is true), or the
// ones of the symbol name ("T", or "T_Method" for a method), under a header
// EXAMPLES. With -all each example comes with its comment, its code and its
// output.
func (s *session) examples(all bool, name string) {
	if !s.opts.Examples || len(s.ps.testFiles) == 0 {
		return
	}
	var list []*godoc.Example
	for _, ex := range godoc.Examples(s.ps.testFiles...) {
		if all || strings.EqualFold(ex.Name, name) {
			list = append(list, ex)
		}
	}
	if len(list) == 0 {
		return
	}
	s.printHeader("EXAMPLES")
	for _, ex := range list {
		fn := "Example" + ex.Name
		if ex.Suffix != "" {
			fn += "_" + ex.Suffix
		}
		s.Printf("%s%s\n", indent, fn)
		if !s.opts.All {
			continue
		}
		if ex.Doc != "" {
			s.ToText(&s.buf, ex.Doc, indent+indent, indent+indent+indent)
		}
		if ex.Code != nil {
			var sb strings.Builder
			if err := formatNode(&sb, s.ps.fset, ex.Code); err == nil {
				body := strings.TrimSuffix(strings.TrimPrefix(sb.String(), "{\n"), "}")
				for line := range strings.Lines(body) {
					if strings.TrimSpace(line) == "" {
						s.Printf("\n")
					} else {
						s.Printf("%s%s", indent+indent, strings.TrimPrefix(line, "\t"))
					}
				}
			}
		}
		if ex.Output != "" || ex.EmptyOutput {
			s.Printf("%s// Output:\n", indent+indent)
			for line := range strings.Lines(ex.Output) {
				s.Printf("%s// %s", indent+indent, line)
			}
		}
		s.newlines(1)
	}
}
