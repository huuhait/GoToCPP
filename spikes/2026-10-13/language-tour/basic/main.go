package main

import (
	"errors"
	"fmt"
	"strings"
)

type User struct {
	Name string
	Age  int
}

func (u User) Greeting() string { return "hi " + u.Name }

func (u *User) Birthday() { u.Age++ }

func find(users []User, name string) (*User, error) {
	for i := range users {
		if strings.EqualFold(users[i].Name, name) {
			return &users[i], nil
		}
	}
	return nil, errors.New("user not found")
}

func main() {
	users := []User{{"Ann", 30}, {"Bob", 25}}

	u, err := find(users, "bob")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	u.Birthday()
	fmt.Println(u.Greeting(), u.Age)

	count := map[string]int{}
	for _, w := range strings.Fields("go cpp go go cpp") {
		count[w]++
	}
	fmt.Println(count)
}
