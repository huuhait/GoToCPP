package types

// Config only declares a type: no function of this package uses the unnamed interface types in
// its fields, so only the declaration can make their C++ structs.
type Config struct {
	Mu    interface{ Lock() }
	Names [2]interface{ Name() string }
}
