// bench: json_unmarshal_struct
// category: data
// size: 100000
// smoke: 100
// twin: no
// about: encoding/json Unmarshal of a small typical document into a struct
package main

import (
	"encoding/json"
	"fmt"
)

type Address struct {
	Street string `json:"street"`
	City   string `json:"city"`
	Zip    string `json:"zip,omitempty"`
}

type Person struct {
	ID       int64             `json:"id"`
	Name     string            `json:"name"`
	Email    string            `json:"email,omitempty"`
	Age      int               `json:"age"`
	Score    float64           `json:"score"`
	Active   bool              `json:"active"`
	Tags     []string          `json:"tags"`
	Address  Address           `json:"address"`
	Settings map[string]string `json:"settings,omitempty"`
	Friends  []int             `json:"friends"`
}

const doc = `{"id":1234567890,"name":"Alice <Admin> & Co","email":"alice@example.com","age":41,"score":98.6,"active":true,"tags":["admin","staff","tést"],"address":{"street":"1 Main St","city":"Springfield","zip":"12345"},"settings":{"lang":"en","theme":"dark","tz":"UTC"},"friends":[1,2,3,5,8,13,21]}`

func main() {
	data := []byte(doc)
	total := 0
	for i := 0; i < benchN; i++ {
		var p Person
		if err := json.Unmarshal(data, &p); err != nil {
			panic(err)
		}
		total += int(p.ID%7) + len(p.Name) + len(p.Tags) + len(p.Settings) + int(p.Address.City[0]%3)
	}
	fmt.Println(total)
}
