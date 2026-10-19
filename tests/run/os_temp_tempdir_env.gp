// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	os.Setenv("TMPDIR", "/custom/tmp")
	fmt.Println(os.TempDir())
	os.Setenv("TMPDIR", "")
	fmt.Println(os.TempDir())
	os.Unsetenv("TMPDIR")
	fmt.Println(os.TempDir())
}

// Output:
// /custom/tmp
// /tmp
// /tmp
