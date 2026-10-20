// phase: 09

// Every Scan method of one Sscan call gets the same fmt.ScanState (one scanner per call).
package main

import "fmt"

var states []fmt.ScanState

type K struct{}

func (k *K) Scan(st fmt.ScanState, verb rune) error {
	states = append(states, st)
	_, err := st.Token(true, nil)
	return err
}

func main() {
	var a, b K
	fmt.Sscan("x y", &a, &b)
	fmt.Println(len(states), states[0] == states[1])
	fmt.Printf("%T\n", states[0])
}

// Output:
// 2 true
// *fmt.ss
