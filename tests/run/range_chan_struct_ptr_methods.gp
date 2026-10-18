// phase: 08

// Methods on values received in a range loop; pointer receivers on a copy; modifying the copy does not
// change what the sender kept.
package main

import "fmt"

type counter struct {
	name string
	n    int
}

func (c *counter) inc() { c.n++ }
func (c counter) show() string { return fmt.Sprint(c.name, "=", c.n) }

func main() {
	orig := counter{"x", 1}
	c := make(chan counter, 3)
	for i := 0; i < 3; i++ {
		c <- orig
	}
	close(c)
	for v := range c {
		v.inc()
		v.inc()
		fmt.Println(v.show(), orig.show())
	}
	pc := make(chan *counter, 2)
	shared := &counter{"p", 0}
	pc <- shared
	pc <- shared
	close(pc)
	for p := range pc {
		p.inc()
	}
	fmt.Println(shared.show())
}

// Output:
// x=3 x=1
// x=3 x=1
// x=3 x=1
// p=2
