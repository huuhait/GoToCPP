// phase: 05

// #Derive(String) on integer types with constants (step 10.10): the name of the constant, or
// Type(n) for another value, as Go's stringer tool does. A value that two constants share gets the
// first name in the order of the declarations; signed, unsigned, small and unexported types work.
// The twin (testdata/derive_twin/derive_enum.go) writes the methods by hand.

package main

import (
	"fmt"
)

#Derive(String)
type Level int

const (
	Debug Level = iota - 1
	Info
	Warn
	Default = Info
	Error   = Warn + 1
	Alias   = Debug
)

#Derive(String)
type Color uint8

const (
	Red Color = iota + 1
	Green
	Blue
	_
	Alpha Color = 200
)

const Hidden Color = 9

#Derive(String)
type big int64

const (
	small big = -1 << 40
	huge  big = 1 << 40
)

#Derive(String)
type Weekday uint

const (
	Monday Weekday = iota
	Tuesday
	Friday = Weekday(4)
)

#Derive(String)
type Flag int8

const (
	Off Flag = 0
	On  Flag = 1
	Neg Flag = -128
)

type Config struct {
	Level Level
	Color Color
}

func main() {
	fmt.Println(Debug, Info, Warn, Default, Error, Alias)
	fmt.Println(Level(-2), Level(0), Level(100))
	fmt.Println(Red, Green, Blue, Alpha, Hidden, Color(4), Color(255))
	fmt.Println(small, huge, big(0), big(1<<62))
	fmt.Println(Monday, Tuesday, Friday, Weekday(3), Weekday(1<<63))
	fmt.Println(Off, On, Neg, Flag(127))
	fmt.Printf("%v %s %d|%5v|%-7s|\n", Warn, Warn, Warn, Info, Info)
	fmt.Printf("%q %x\n", Error, Info)
	fmt.Println(Info.String() + "/" + Color(7).String())
	fmt.Println([]Level{Debug, Error, Level(9)}, map[Level]string{Info: "i", Debug: "d"})
	fmt.Println(Config{Warn, Green})
	fmt.Printf("%+v %v\n", Config{Info, Blue}, &Config{Debug, Red})
	var s fmt.Stringer = Warn
	fmt.Println(s.String(), s)
	keys := []Weekday{Friday, Monday, Tuesday}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	fmt.Println(keys)
}

// Output:
// Debug Info Warn Info Error Debug
// Level(-2) Info Level(100)
// Red Green Blue Alpha Hidden Color(4) Color(255)
// small huge big(0) big(4611686018427387904)
// Monday Tuesday Friday Weekday(3) Weekday(9223372036854775808)
// Off On Neg Flag(127)
// Warn Warn 1| Info|Info   |
// "Error" 496e666f
// Info/Color(7)
// [Debug Error Level(9)] map[Debug:d Info:i]
// {Warn Green}
// {Level:Info Color:Blue} &{Debug Red}
// Warn Warn
// [Monday Tuesday Friday]
