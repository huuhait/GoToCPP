// phase: 09

package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

var saved fmt.State

type F struct{}

func (F) Format(s fmt.State, c rune) { saved = s }

func main() {
	_ = fmt.Sprintf("%v", F{})
	n, err := saved.Write([]byte("xy"))
	m, err2 := io.WriteString(saved, "abc")
	os.Stdout.WriteString(strconv.Itoa(n) + " " + strconv.FormatBool(err == nil) + " " + strconv.Itoa(m) + " " + strconv.FormatBool(err2 == nil) + "\n")
}

// Output:
// 2 true 3 true
