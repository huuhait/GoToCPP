// phase: 08

// A Duration through fmt: %v, %s, %d, %T and friends take the Stringer or the integer path.
package main

import (
	"fmt"
	"time"
)

type job struct {
	Name    string
	Timeout time.Duration
	Retries []time.Duration
}

type wrapper struct {
	d time.Duration
	p *time.Duration
}

func main() {
	d := 1500 * time.Millisecond
	fmt.Println(d)
	fmt.Printf("%v|%s|%d|%T|%q|%x|%08d|%+v|%#v\n", d, d, d, d, d, d, d, d, d)
	fmt.Printf("%10v|%-10v|%10s|\n", d, d, d)
	fmt.Printf("%v %v\n", []time.Duration{time.Second, time.Minute, 90 * time.Minute}, [2]time.Duration{1, 2})
	fmt.Printf("%v\n", map[string]time.Duration{"a": time.Second, "b": time.Hour})
	j := job{"x", 2 * time.Second, []time.Duration{time.Millisecond, 3 * time.Hour}}
	fmt.Printf("%v\n%+v\n", j, j)
	fmt.Println(fmt.Sprint(d), fmt.Sprintf("%v", time.Duration(0)))
	var np *time.Duration
	pd := &d
	fmt.Println(np == nil, *pd, pd != nil)
	w := wrapper{d: time.Minute}
	fmt.Printf("%v\n", w.d)
	fmt.Printf("%.2f seconds, %d ms\n", d.Seconds(), d.Milliseconds())
	var i any = d
	if s, ok := i.(fmt.Stringer); ok {
		fmt.Println("stringer:", s.String())
	}
	if dd, ok := i.(time.Duration); ok {
		fmt.Println("duration:", dd)
	}
}

// Output:
// 1.5s
// 1.5s|1.5s|1500000000|time.Duration|"1.5s"|312e3573|1500000000|1.5s|1500000000
//       1.5s|1.5s      |      1.5s|
// [1s 1m0s 1h30m0s] [1ns 2ns]
// map[a:1s b:1h0m0s]
// {x 2s [1ms 3h0m0s]}
// {Name:x Timeout:2s Retries:[1ms 3h0m0s]}
// 1.5s 0s
// true 1.5s true
// 1m0s
// 1.50 seconds, 1500 ms
// stringer: 1.5s
// duration: 1.5s
