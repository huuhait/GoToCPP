package main

import "fmt"

func cleanup(name string) { fmt.Println("cleanup", name) }

func work(fail bool) (err error) {
	defer cleanup("outer")
	defer func() {
		if fail {
			err = fmt.Errorf("work failed")
		}
	}()
	if fail {
		return nil
	}
	return nil
}

func main() {
	fmt.Println(work(true))
}
