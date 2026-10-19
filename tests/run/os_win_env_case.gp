// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
)

func e(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

func count(prefix string) int {
	n := 0
	for _, kv := range os.Environ() {
		if len(kv) > len(prefix) && kv[len(prefix)] == '=' && eq(kv[:len(prefix)], prefix) {
			n++
		}
	}
	return n
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func eq(a, b string) bool { return lower(a) == lower(b) }

// Environment variable names are case-insensitive on Windows (the system's block), and Environ
// reports a variable once, with the case it was last set with.
func main() {
	_, ok := os.LookupEnv("pAtH")
	fmt.Println("PATH found as pAtH:", ok)
	fmt.Println(os.Setenv("GPWIN_Case", "one"))
	fmt.Printf("%q %q %q\n", os.Getenv("GPWIN_Case"), os.Getenv("gpwin_case"), os.Getenv("GPWIN_CASE"))
	fmt.Println(os.Setenv("gpwin_CASE", "two"))
	fmt.Printf("%q %q\n", os.Getenv("GPWIN_Case"), os.Getenv("gpwin_case"))
	fmt.Println("entries:", count("GPWIN_CASE"))
	fmt.Println(os.Unsetenv("GPWIN_case"))
	_, ok = os.LookupEnv("GPWIN_Case")
	fmt.Println("after Unsetenv:", ok, count("GPWIN_CASE"))
	// An empty value is a value.
	fmt.Println(os.Setenv("GPWIN_EMPTY", ""))
	v, ok := os.LookupEnv("gpwin_empty")
	fmt.Printf("%q %v\n", v, ok)
	// Names that the system refuses.
	fmt.Println(e(os.Setenv("", "x")))
	fmt.Println(e(os.Setenv("A=B", "x")))
	fmt.Println(e(os.Setenv("GPWIN_NUL", "a\x00b")))
	fmt.Println(e(os.Setenv("GPWIN_N\x00UL", "ab")))
	// The drive variables of the system ("=C:") are in Environ, and Expand sees ordinary names.
	n := 0
	for _, kv := range os.Environ() {
		if len(kv) > 1 && kv[0] == '=' {
			n++
		}
	}
	fmt.Println("has =X: entries:", n > 0)
	os.Setenv("GPWIN_X", "val")
	fmt.Println(os.ExpandEnv("$gpwin_x ${GPWIN_x} $GPWIN_NONE|"))
}

// Output:
// PATH found as pAtH: true
// <nil>
// "one" "one" "one"
// <nil>
// "two" "two"
// entries: 1
// <nil>
// after Unsetenv: false 0
// <nil>
// "" true
// setenv: The parameter is incorrect.
// setenv: The parameter is incorrect.
// setenv: invalid argument
// setenv: invalid argument
// has =X: entries: true
// val val |
