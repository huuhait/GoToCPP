package main

import "fmt"

func hello(name string) (string, error) {
	if _, err := fmt.Println("hello", name); err != nil {
		return "", err
	}
	if _, err := fmt.Printf("%d args\n", 1); err != nil {
		return "", err
	}
	return "done " + name, nil
}

func main() {
	fmt.Println(hello("go"))
}
