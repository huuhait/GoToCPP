// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
)

// Windows file and variable names and values are UTF-16 that may hold unpaired surrogates; Go keeps
// them as WTF-8 strings, and they come back from the environment as the bytes they went in with.
func main() {
	for _, v := range []string{"a\xed\xa0\x80b", "\xed\xb0\x80", "x\xed\xa0\x80\xed\xb0\x80y", "plain \U0001F642"} {
		fmt.Println(os.Setenv("GPWIN_WTF8", v))
		got := os.Getenv("GPWIN_WTF8")
		fmt.Printf("%q %d %v\n", got, len(got), got == v)
		found := false
		for _, kv := range os.Environ() {
			if kv == "GPWIN_WTF8="+v {
				found = true
			}
		}
		fmt.Println("in Environ:", found)
	}
	// A name with an unpaired surrogate.
	k := "GPWIN_K\xed\xa0\x80"
	fmt.Println(os.Setenv(k, "v"))
	fmt.Printf("%q %v\n", os.Getenv(k), os.Getenv(k) == "v")
	fmt.Println(os.Unsetenv(k))
	_, ok := os.LookupEnv(k)
	fmt.Println(ok)
}

// Output:
// <nil>
// "a\xed\xa0\x80b" 5 true
// in Environ: true
// <nil>
// "\xed\xb0\x80" 3 true
// in Environ: true
// <nil>
// "x𐀀y" 6 false
// in Environ: false
// <nil>
// "plain 🙂" 10 true
// in Environ: true
// <nil>
// "v" true
// <nil>
// false
