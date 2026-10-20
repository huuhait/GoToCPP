// bench: json_doc
// category: data
// size: 6
// smoke: 1
// twin: no
// about: encoding/json on a document of about 1 MB (records with strings, numbers, arrays and nested objects): Marshal, Unmarshal into structs, Unmarshal into any, Indent, Valid, benchN rounds
package main

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type Item struct {
	SKU   string  `json:"sku"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

type Record struct {
	ID      int               `json:"id"`
	Name    string            `json:"name"`
	Email   string            `json:"email"`
	Active  bool              `json:"active"`
	Balance float64           `json:"balance"`
	Tags    []string          `json:"tags"`
	Items   []Item            `json:"items"`
	Meta    map[string]string `json:"meta"`
	Note    *string           `json:"note"`
}

var seed uint64 = 99

func rnd() uint64 {
	seed = seed*6364136223846793005 + 1442695040888963407
	return seed >> 33
}

func word(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + rnd()%26)
	}
	return string(b)
}

func main() {
	recs := make([]Record, 2200)
	for i := range recs {
		r := &recs[i]
		r.ID = i
		r.Name = word(5+int(rnd()%8)) + " " + word(6)
		r.Email = word(8) + "@" + word(6) + ".com"
		r.Active = rnd()%2 == 0
		r.Balance = float64(rnd()%1000000) / 100
		for j := 0; j < 1+int(rnd()%4); j++ {
			r.Tags = append(r.Tags, word(4))
		}
		for j := 0; j < 1+int(rnd()%3); j++ {
			r.Items = append(r.Items, Item{"SKU-" + strconv.Itoa(int(rnd()%100000)), int(rnd() % 10), float64(rnd()%50000) / 100})
		}
		r.Meta = map[string]string{"k" + strconv.Itoa(int(rnd()%5)): word(7), "region": word(3)}
		if rnd()%3 == 0 {
			s := word(20) + "\n\"quoted\" <b>"
			r.Note = &s
		}
	}
	data, err := json.Marshal(recs)
	if err != nil {
		panic(err)
	}
	total := len(data)
	for round := 0; round < benchN; round++ {
		out, err := json.Marshal(recs)
		if err != nil {
			panic(err)
		}
		total += len(out)

		var back []Record
		if err := json.Unmarshal(data, &back); err != nil {
			panic(err)
		}
		total += len(back) + back[len(back)/2].ID

		var anyv any
		if err := json.Unmarshal(data, &anyv); err != nil {
			panic(err)
		}
		total += len(anyv.([]any))

		if !json.Valid(data) {
			panic("invalid")
		}
		ind, err := json.MarshalIndent(anyv, "", "  ")
		if err != nil {
			panic(err)
		}
		total += len(ind)
	}
	fmt.Println(total)
}
