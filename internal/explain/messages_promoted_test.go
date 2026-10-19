package explain

import "testing"

// TestPromotedMessages: each message of the second pass for promoted C++ members (internal/check/cppmembers.go)
// has its own entry, found before the general "interop/no-member".
func TestPromotedMessages(t *testing.T) {
	for msg, want := range map[string]string{
		"s.nosuch undefined (type S has no field or method nosuch; std.vector[int] has no member nosuch)":                                "interop/promoted-missing",
		"s.nosuch undefined (type S has no field or method nosuch; none of the embedded C++ types A, B has a member nosuch)":             "interop/promoted-missing-several",
		"s.MAC undefined (type S has no field or method MAC; C has no member MAC) (MAC is a macro of the headers)":                       "interop/promoted-macro",
		"s.Ty undefined (type S has no field or method Ty; C has no member Ty) (Ty is a type of the class, not a member)":                "interop/promoted-type",
		"ambiguous selector s.size (std.vector[int].size and std.string.size)":                                                           "interop/promoted-ambiguous",
		"ambiguous selector s.x (D has a member x in several bases)":                                                                     "interop/promoted-bases",
		"s.size is the member of std.vector[int], which is shallower than the Go member size (Go's rule): name the field, s.vector.size": "interop/promoted-shadow",
		"s.priv undefined (the member priv of C is private)":                                                                             "interop/promoted-private",
		"s.prot undefined (the member prot of C is protected)":                                                                           "interop/promoted-protected",
		"s.del is a deleted function of C":                                                                                               "interop/promoted-deleted",
	} {
		var got string
		for _, m := range Messages() {
			if m.Matches(msg) {
				got = m.ID
				break
			}
		}
		if got != want {
			t.Errorf("%q is explained by %q, want %q", msg, got, want)
		}
	}
}
