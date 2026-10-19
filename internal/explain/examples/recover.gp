package main

import "fmt"

func safeDiv(a, b int) (q int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	return a / b, nil
}

func main() {
	fmt.Println(safeDiv(7, 2))
	fmt.Println(safeDiv(1, 0))
}
