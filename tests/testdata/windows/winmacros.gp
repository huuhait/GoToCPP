// phase: 14

// Names of a gocpp program that are macros of <windows.h> (step 14.4, ADR-082). The macro protection of
// Phase 04 renames a name of the package that a header makes a macro (a _ is added), so the program
// means what it says. NOMINMAX (a default for windows) keeps min and max out of it, and UNICODE makes
// the name MessageBox mean MessageBoxW.

//gocpp:link user32

package main

import (
	"fmt"

	"<windows.h>"
)

const (
	ERROR    = 11
	DELETE   = 12
	CONST    = 13
	TRUE     = 14
	FALSE    = 15
	MAX_PATH = 16
	INFINITE = 17
)

var (
	near  = 21
	far   = 22
	small = 23
	IN    = 24
	OUT   = 25
)

type VOID struct{ OPTIONAL int }

type BOOL int

type Message struct {
	min, max int
	SendMessage string
}

func (m Message) GetObject() string { return "GetObject of " + m.SendMessage }

func (m *Message) CreateWindow(n int) int { return n + m.min + m.max }

func DrawText(s string) string { return "DrawText:" + s }

func LoadImage() int { return 31 }

func RGB(r, g, b int) int { return r | g<<8 | b<<16 }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	fmt.Println(ERROR, DELETE, CONST, TRUE, FALSE, MAX_PATH, INFINITE)
	fmt.Println(near, far, small, IN, OUT)
	v := VOID{OPTIONAL: 5}
	fmt.Println(v.OPTIONAL, BOOL(6))
	m := Message{min: 1, max: 2, SendMessage: "hi"}
	fmt.Println(m.GetObject(), m.CreateWindow(10), m)
	fmt.Println(DrawText("x"), LoadImage(), RGB(1, 2, 3), min(3, 4), max(3, 4))

	// The macros of the header, used through the header package: MessageBox is MessageBoxW with UNICODE.
	fmt.Println(windows.MAX_PATH, windows.TRUE, windows.FALSE, windows.INFINITE, windows.DELETE)
	fmt.Println(int(windows.GetSystemMetrics(windows.SM_CXSCREEN)) >= 0, windows.RGB(1, 2, 3), windows.LOWORD(0x12345678), windows.HIWORD(0x12345678))
	fmt.Println(windows.MessageBox == windows.MessageBoxW)
}
