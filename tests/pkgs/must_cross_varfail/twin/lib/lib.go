package lib

import (
	"fmt"
	"strconv"
)

var Port = mustAtoi(portText)

func mustAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return n
}

var portText = text()

func text() string {
	fmt.Println("lib: text")
	return "80x"
}

func init() {
	fmt.Println("lib: init (not reached)")
}
