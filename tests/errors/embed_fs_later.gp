// phase: 10

// //go:embed on an embed.FS is not available until Phase 09 has io/fs (Phase 10 step 10.8): string and
// []byte variables work.

package main

import (
	"embed"
	"fmt"
)

//go:embed embeddata/x.txt
var fs embed.FS // ERROR "embed.FS is not available yet \(Phase 09"

//go:embed embeddata
var dir embed.FS // ERROR "embed.FS is not available yet \(Phase 09"

//go:embed embeddata/x.txt
var ok string

func main() { fmt.Println(fs, dir, ok) }
