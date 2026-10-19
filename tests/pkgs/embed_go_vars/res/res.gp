package res

import _ "embed"

// The files are those next to this source file, not the module's root.
//
//go:embed x.txt
var X string

//go:embed x.txt
var XB []byte
