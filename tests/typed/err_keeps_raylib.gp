// phase: 12
// pkg-config: raylib
// error: InitWindow keeps the pointer that it is given

// A function that keeps the pointer is not given a gocpp string by itself: raylib keeps the title
// of the window, and the bytes of a gocpp string live only for the statement.
//
//gocpp:pkg-config raylib
package main

import (
	"fmt"

	rl "<raylib.h>"
)

func main() {
	title := "game " + fmt.Sprint(1)
	rl.InitWindow(320, 200, title)
}
