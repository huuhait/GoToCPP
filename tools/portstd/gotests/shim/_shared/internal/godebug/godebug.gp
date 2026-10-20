// Package godebug stands in for internal/godebug in the copies of ported packages that Go's tests run in (a program cannot import
// Go's internal packages): a setting whose value is always empty, as in gocpp's own internal/godebug when GODEBUG is empty.
package godebug

// A Setting is a single setting in the $GODEBUG environment variable.
type Setting struct{ name string }

// New returns a new Setting for the $GODEBUG setting with the given name.
func New(name string) *Setting { return &Setting{name} }

// Name returns the name of the setting.
func (s *Setting) Name() string { return s.name }

// Value returns the current value for the GODEBUG setting s (always empty).
func (s *Setting) Value() string { return "" }

// String returns a printable form for the setting: name=value.
func (s *Setting) String() string { return s.name + "=" }

// IncNonDefault increments the non-default behavior counter (nothing here).
func (s *Setting) IncNonDefault() {}

// Undocumented reports whether this is an undocumented setting.
func (s *Setting) Undocumented() bool { return false }
