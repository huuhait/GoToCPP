// phase: 03

// Named types over numbers (step 3.9): a temperature table with Celsius and Fahrenheit (their
// own types, converted explicitly), untyped constants that take the named type, an enum of a
// named int with a switch, and named counters that wrap like their underlying types.
package main

import "fmt"

type Celsius float64
type Fahrenheit float64

const (
	Freezing Celsius = 0
	Boiling  Celsius = 100
)

func CToF(c Celsius) Fahrenheit { return Fahrenheit(c*9/5 + 32) }
func FToC(f Fahrenheit) Celsius { return Celsius((f - 32) * 5 / 9) }

type Weekday int

const (
	Sunday Weekday = iota
	Monday
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
)

func kind(d Weekday) string {
	switch d {
	case Saturday, Sunday:
		return "weekend"
	case Friday:
		return "almost"
	}
	return "work"
}

type Tick uint8

func main() {
	for c := Freezing; c <= Boiling; c += 25 {
		fmt.Println(c, CToF(c))
	}
	fmt.Println(FToC(-40), FToC(212) == Boiling, CToF(Boiling)-CToF(Freezing))
	for d := range Weekday(7) {
		fmt.Println(d, kind(d), (d+1)%7)
	}
	var t Tick = 250
	for range 10 {
		t++
	}
	fmt.Println(t, t*2, t-10, t<<4, t>>1)
	avg := (Celsius(20.5) + Celsius(22) + Celsius(19)) / 3
	fmt.Printf("%v %f %d\n", avg, avg, int(avg))
}

// Output:
// 0 32
// 25 77
// 50 122
// 75 167
// 100 212
// -40 true 180
// 0 weekend 1
// 1 work 2
// 2 work 3
// 3 work 4
// 4 work 5
// 5 almost 6
// 6 weekend 0
// 4 8 250 64 2
// 20.5 20.500000 20
