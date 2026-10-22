package lower

import "github.com/huuhait/gocpp/internal/typedinterop/adapt"

// Read-only views of the tables of the keeps rules (internal/typedinterop/adapt/keeps.go) for tools
// that must apply the same rules without lowering anything: gocpp vet (internal/vet, analyzer cstr)
// warns about C.cstr strings given to a function that keeps the pointer, and a test there compares
// its copy with these, so that the two lists cannot drift. They are copies: a caller can't change
// the tables.

// KeeperFuncs returns the functions whose const char* parameters the callee keeps, by the last
// name of the function, with why.
func KeeperFuncs() map[string]string { return adapt.KeeperFuncs() }

// DestructorArgs returns the index of the destructor argument of the SQLite functions that are
// given bytes.
func DestructorArgs() map[string]int { return adapt.DestructorArgs() }

// ConventionWords returns the words in a function's name that say that it keeps its pointers.
func ConventionWords() []string { return adapt.ConventionWords() }
