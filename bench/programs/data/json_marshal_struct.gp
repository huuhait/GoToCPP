// bench: json_marshal_struct
// category: data
// size: 200000
// smoke: 200
// twin: no
// about: encoding/json Marshal of a small typical struct (nested struct, slice, map, optional fields, floats and strings)
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

func main() {
	p := Person{
		ID: 1234567890, Name: "Alice <Admin> & Co", Email: "alice@example.com", Age: 41, Score: 98.6, Active: true,
		Tags:     []string{"admin", "staff", "tést"},
		Address:  Address{"1 Main St", "Springfield", "12345"},
		Settings: map[string]string{"theme": "dark", "lang": "en", "tz": "UTC"},
		Friends:  []int{1, 2, 3, 5, 8, 13, 21},
	}
	total := 0
	for i := 0; i < benchN; i++ {
		p.ID = int64(i)
		p.Score = float64(i%1000) / 8
		b, err := json.Marshal(&p)
		if err != nil {
			panic(err)
		}
		total += len(b) + int(b[len(b)/2])
	}
	fmt.Println(total)
}
