// The $GODEBUG a program starts with (tests/godebug_env_test.go runs it with exact environments,
// duplicates kept, and compares it with the same program built by real Go): internal/godebug
// reads it for a package variable, an init function, a goroutine that init starts and main, the
// first entry of the name winning (without case on Windows), and then os.Setenv and os.Unsetenv
// change it (on Windows, Unsetenv removes one variable of the name, as Go).
package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
)

// seeded says whether rand.Seed seeds the global generator: math/rand's randseednop setting is 0.
func seeded() bool {
	rand.Seed(42)
	a := rand.Int63()
	rand.Seed(42)
	b := rand.Int63()
	return a == b
}

var atVar = seeded()
var atInit bool
var fromInit = make(chan bool, 1)

func init() {
	atInit = seeded()
	go func() { fromInit <- seeded() }()
}

func show(what string) {
	var es []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(strings.ToUpper(e), "GODEBUG=") {
			es = append(es, e)
		}
	}
	v, ok := os.LookupEnv("GODEBUG")
	fmt.Printf("%s: GODEBUG=%q set=%v env=%q seeded=%v\n", what, v, ok, es, seeded())
}

func main() {
	fmt.Println("var:", atVar, "init:", atInit, "init goroutine:", <-fromInit)
	show("main")
	os.Setenv("OTHER", "1")
	show("other setenv")
	os.Unsetenv("godebug")
	show("unsetenv godebug")
	os.Unsetenv("GODEBUG")
	show("unsetenv GODEBUG")
	os.Setenv("GODEBUG", "")
	show("setenv empty")
	os.Setenv("godebug", "randseednop=0")
	show("setenv godebug")
	os.Setenv("GODEBUG", "randseednop=0")
	show("setenv GODEBUG")
}
