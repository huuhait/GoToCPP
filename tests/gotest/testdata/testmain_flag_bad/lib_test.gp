package lib

import (
	"flag"
	"fmt"
	"os"
	"testing"
)

var n = flag.Int("n", 1, "a number")
var name = flag.String("name", "none", "a name")

// The idiom: TestMain parses the flags (the test binary's and the program's) with package flag.
func TestMain(m *testing.M) {
	flag.Parse()
	fmt.Println("n =", *n, "name =", *name, "first arg =", flag.Arg(0), "short =", testing.Short(), "verbose =", testing.Verbose())
	os.Exit(m.Run())
}

func TestOne(t *testing.T) { t.Log("n is", *n) }
