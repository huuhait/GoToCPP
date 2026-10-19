// phase: 05

package main

import (
	"errors"
	"fmt"
)

func f(a int, b string, c bool) (err error) {
	defer errors.Wrapf(&err, "a=%d b=%s c=%t pct=%d%%", a, b, c, 50)
	return errors.New("boom")
}

func main() {
	fmt.Println(f(1, "two", true))
}

// Output:
// a=1 b=two c=true pct=50%: boom
