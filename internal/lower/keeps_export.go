package lower

import "maps"

// Read-only views of the tables of keeps.go for tools that must apply the same rules without
// lowering anything: gocpp vet (internal/vet, analyzer cstr) warns about C.cstr strings given to a
// function that keeps the pointer, and a test there compares its copy with these, so that the two
// lists cannot drift. They are copies: a caller can't change the lowering's tables.

// KeeperFuncs returns keeperFuncs: the functions whose const char* parameters the callee keeps, by
// the last name of the function, with why.
func KeeperFuncs() map[string]string { return maps.Clone(keeperFuncs) }

// DestructorArgs returns destructorArg: the index of the destructor argument of the SQLite
// functions that are given bytes.
func DestructorArgs() map[string]int { return maps.Clone(destructorArg) }

// ConventionWords returns conventionWords: the words in a function's name that say that it keeps
// its pointers.
func ConventionWords() []string { return append([]string(nil), conventionWords...) }
