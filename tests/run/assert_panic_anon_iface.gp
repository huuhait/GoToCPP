// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: interface \{ Alpha\(\.\.\.int\) string; Zeta\(int, int\) \(int, error\) \} is main\.T, not main\.U

// The panic names an unnamed interface type as Go does: its methods flattened and sorted, no
// parameter names, ...int for a variadic parameter (step 5.5).
package main

import "fmt"

type T struct{}
type U struct{}

func (T) Zeta(a, b int) (int, error) { return 0, nil }
func (T) Alpha(xs ...int) string      { return "" }

func (U) Zeta(a, b int) (int, error) { return 0, nil }
func (U) Alpha(xs ...int) string      { return "" }

func main() {
	var x interface {
		Zeta(x, y int) (int, error)
		Alpha(rest ...int) string
	} = T{}
	fmt.Println(x.Alpha(1, 2))
	u := x.(U)
	fmt.Println(u)
}

// Output:
