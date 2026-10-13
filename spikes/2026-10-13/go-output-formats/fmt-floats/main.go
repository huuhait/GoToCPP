package main

import "fmt"

func main() {
	fmt.Println(123456.0, 1234567.0, 1000000.0, 1e20, 1e21, 0.0001, 0.00001, 3.0, 2.5, float32(0.1))
	var f float64 = 1e300
	fmt.Println(int64(f), int64(-f), int32(f))
}
