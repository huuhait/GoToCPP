// #Derive(String) on integer types with constants (step 10.10): the name of the constant, or
// Type(n) for another value, as Go's stringer tool does. A value that two constants share gets the
// first name in the order of the declarations; signed, unsigned, small and unexported types work.

package main

import (
	"fmt"
	"strconv"
)

type Level int

const (
	Debug Level = iota - 1
	Info
	Warn
	Default = Info
	Error   = Warn + 1
	Alias   = Debug
)

type Color uint8

const (
	Red Color = iota + 1
	Green
	Blue
	_
	Alpha Color = 200
)

const Hidden Color = 9

type big int64

const (
	small big = -1 << 40
	huge  big = 1 << 40
)

type Weekday uint

const (
	Monday Weekday = iota
	Tuesday
	Friday = Weekday(4)
)

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

func (l Level) String() string {
	switch l {
	case -1:
		return "Debug"
	case 0:
		return "Info"
	case 1:
		return "Warn"
	case 2:
		return "Error"
	}
	return "Level(" + strconv.Itoa(int(l)) + ")"
}

func (c Color) String() string {
	switch c {
	case 1:
		return "Red"
	case 2:
		return "Green"
	case 3:
		return "Blue"
	case 200:
		return "Alpha"
	case 9:
		return "Hidden"
	}
	return "Color(" + strconv.FormatUint(uint64(c), 10) + ")"
}

func (b big) String() string {
	switch b {
	case -1 << 40:
		return "small"
	case 1 << 40:
		return "huge"
	}
	return "big(" + strconv.FormatInt(int64(b), 10) + ")"
}

func (d Weekday) String() string {
	switch d {
	case 0:
		return "Monday"
	case 1:
		return "Tuesday"
	case 4:
		return "Friday"
	}
	return "Weekday(" + strconv.FormatUint(uint64(d), 10) + ")"
}

func (f Flag) String() string {
	switch f {
	case 0:
		return "Off"
	case 1:
		return "On"
	case -128:
		return "Neg"
	}
	return "Flag(" + strconv.FormatInt(int64(f), 10) + ")"
}
