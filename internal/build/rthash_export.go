package build

// RuntimeHash returns the hash of the runtime embedded in this gocpp binary (16 hex digits): the
// second part of the runtime's folder name in the cache, <version>-<hash>. gocpp version prints
// it (ADR-009, step 15.13).
func RuntimeHash() string { return rtHash() }
