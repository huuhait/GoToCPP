// phase: 09
// encoding/json from many goroutines at once: the encoder and field caches (sync.Map) are filled by several goroutines for the same types, and every result must equal the sequential one.

package main

import (
	"encoding/json"
	"fmt"
	"sync"
)

type Leaf struct {
	N int    `json:"n"`
	S string `json:"s,omitempty"`
}

type A struct {
	ID   int               `json:"id"`
	Leaf Leaf              `json:"leaf"`
	List []Leaf            `json:"list"`
	M    map[string]Leaf   `json:"m"`
	P    *Leaf             `json:"p"`
	Any  any               `json:"any"`
}

type B struct {
	A
	Extra [2]float64 `json:"extra"`
	Name  string     `json:"name"`
}

type Outer struct {
	X int `json:"x"`
	Y []B `json:"y"`
}

func sample(i int) Outer {
	b := B{A: A{ID: i, Leaf: Leaf{i, "leaf"}, List: []Leaf{{1, "a"}, {2, ""}}, M: map[string]Leaf{"k": {i, "v"}}, P: &Leaf{N: i * 2}, Any: []int{i, i + 1}}, Extra: [2]float64{1.5, float64(i)}, Name: fmt.Sprint("n", i)}
	return Outer{X: i, Y: []B{b, b}}
}

func main() {
	const workers = 16
	const rounds = 40
	want := make([]string, workers)
	for i := range want {
		b, err := json.Marshal(sample(i))
		if err != nil {
			panic(err)
		}
		want[i] = string(b)
	}
	var wg sync.WaitGroup
	errs := make([]string, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				b, err := json.Marshal(sample(w))
				if err != nil || string(b) != want[w] {
					errs[w] = fmt.Sprint("marshal mismatch ", w, r, err)
					return
				}
				var c Outer
				if err := json.Unmarshal(b, &c); err != nil {
					errs[w] = fmt.Sprint("unmarshal ", err)
					return
				}
				b2, err := json.Marshal(c)
				if err != nil || string(b2) != want[w] {
					errs[w] = fmt.Sprint("round trip mismatch ", w, r, err)
					return
				}
				var anyv any
				if err := json.Unmarshal(b, &anyv); err != nil {
					errs[w] = fmt.Sprint("unmarshal any ", err)
					return
				}
				if _, err := json.MarshalIndent(anyv, "", " "); err != nil {
					errs[w] = fmt.Sprint("indent ", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	bad := 0
	for _, e := range errs {
		if e != "" {
			fmt.Println(e)
			bad++
		}
	}
	fmt.Println("workers:", workers, "mismatches:", bad)
}

// Output:
// workers: 16 mismatches: 0
