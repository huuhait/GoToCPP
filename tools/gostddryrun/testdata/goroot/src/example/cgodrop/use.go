package cgodrop

// Use calls a function declared in a cgo file, which CGO_ENABLED=0 drops.
func Use() int { return FromCgo() }
