// The twin of race_derive_clone.gp: Clone written by hand, on the line range the // Stderr: regexp accepts.
package main

import "fmt"

type Table struct {
	Rows []int
}

func (t Table) Clone() Table {
	out := t
	if t.Rows != nil {
		out.Rows = make([]int, len(t.Rows))
		copy(out.Rows, t.Rows)
	}
	return out
}

func main() {
	t := Table{Rows: make([]int, 8)}
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			t.Rows[i%8] = i
		}
		done <- true
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_ = t.Clone()
		}
		done <- true
	}()
	<-done
	<-done
	fmt.Println("done")
}
