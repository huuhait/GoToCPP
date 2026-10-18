package ext

import (
	"fmt"

	"example.org/ext/text"
)

func Shout(s string) string { return text.Upper(s) + "!" }

func init() { fmt.Println("init ext") }
