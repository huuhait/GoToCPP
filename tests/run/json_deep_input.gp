// phase: 09
// divergence: D24

// Hostile nesting for encoding/json (step 9.11 review): a document or a value nested far deeper than the stack can hold is a clean error, never
// "fatal error: stack overflow". Go grows its stacks, so it decodes up to its limit of 10000 and encodes what it is given; gocpp's stacks are fixed
// (D24), so here the decoder and the encoder ask the runtime for the room that is left (patches/encoding__json.rules) and report
// "exceeded max depth" when there is none. The battery runs on a goroutine (8 MiB), and the part of it whose answer does not depend on the stack
// on the main goroutine too (its stack grows up to 1 GiB, D7); all the numbers are far from the limits of every compiler and mode (-O0, -O2,
// -asan: 200 levels succeed everywhere; 100000 and 1000000 fail everywhere).

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Rec struct {
	A *Rec `json:"a"`
}

type Rec2 struct {
	K []Rec2 `json:"k"`
}

// Types that nest through a slice or a map alone (no pointer, no interface in the cycle).
type S []S

type M map[string]M

type Node struct {
	Next *Node
	V    int
}

// A cycle through a type that has #Derive(JSON): reported as Go reports it for the plain type.
#Derive(JSON)
type DNode struct {
	Next *DNode `json:"next"`
	V    int    `json:"v"`
}

func class(err error) string {
	if err == nil {
		return "ok"
	}
	var se *json.SyntaxError
	var ue *json.UnsupportedValueError
	switch {
	case errors.As(err, &se) && strings.Contains(se.Error(), "exceeded max depth"):
		return "too deep"
	case errors.As(err, &ue) && strings.Contains(ue.Error(), "exceeded max depth"):
		return "too deep"
	}
	return "other: " + err.Error()
}

func repeat(n int, open, mid, closing string) string {
	return strings.Repeat(open, n) + mid + strings.Repeat(closing, n)
}

func battery(who string, sizes, marshalSizes []int) {
	for _, n := range sizes {
		objs := repeat(n, `{"a":`, "null", "}")
		kids := repeat(n, `{"k":[`, "null", "]}")
		arrs := repeat(n, "[", "1", "]")
		pr := func(what string, err error) { fmt.Printf("%s %7d %-26s %s\n", who, n, what, class(err)) }

		if n == 9999 {
			// Under Go's limit of 10000, so the scanner accepts it, but no build holds 9999 levels of a recursive struct on the stack (the most is about
			// 4000, with g++ -release): the decoder's stack guard answers. (Into any the figure depends on the mode, so it is not printed.)
			var r Rec
			pr("Unmarshal Rec", json.Unmarshal([]byte(objs), &r))
			var r2 Rec2
			pr("Unmarshal Rec2", json.Unmarshal([]byte(kids), &r2))
			continue
		}
		var a any
		pr("Unmarshal any", json.Unmarshal([]byte(arrs), &a))
		a = nil
		pr("Unmarshal any (objects)", json.Unmarshal([]byte(objs), &a))
		if n == 1000000 {
			// A million levels: a few of the operations (the sanitizers are slow, and the rest is the same code).
			var r Rec
			pr("Unmarshal Rec", json.Unmarshal([]byte(objs), &r))
			var raw json.RawMessage
			pr("Unmarshal RawMessage", json.Unmarshal([]byte(arrs), &raw))
			fmt.Printf("%s %7d %-26s %v\n", who, n, "Valid", json.Valid([]byte(arrs)))
			continue
		}
		var m map[string]any
		pr("Unmarshal map", json.Unmarshal([]byte(objs), &m))
		var r Rec
		pr("Unmarshal Rec", json.Unmarshal([]byte(objs), &r))
		var r2 Rec2
		pr("Unmarshal Rec2", json.Unmarshal([]byte(kids), &r2))
		var raw json.RawMessage
		pr("Unmarshal RawMessage", json.Unmarshal([]byte(arrs), &raw))
		var num json.Number
		pr("Unmarshal Number", json.Unmarshal([]byte(arrs), &num))
		var ignore struct{ X int }
		pr("Unmarshal skipped value", json.Unmarshal([]byte(objs), &ignore))
		fmt.Printf("%s %7d %-26s %v\n", who, n, "Valid", json.Valid([]byte(arrs)))
		var buf bytes.Buffer
		pr("Compact", json.Compact(&buf, []byte(arrs)))
		buf.Reset()
		pr("Indent", json.Indent(&buf, []byte(arrs), "", " "))
		dec := json.NewDecoder(strings.NewReader(arrs))
		tokens := 0
		var terr error
		for {
			_, err := dec.Token()
			if err != nil {
				terr = err
				break
			}
			tokens++
		}
		fmt.Printf("%s %7d %-26s %d tokens, then %v\n", who, n, "Decoder.Token", tokens, terr)
		pr("Decoder.Decode any", json.NewDecoder(strings.NewReader(arrs)).Decode(&a))
	}

	// Values nested deeply, built by the program.
	for _, n := range marshalSizes {
		var head *Node
		for i := 0; i < n; i++ {
			head = &Node{head, i}
		}
		var nested any = 1
		for i := 0; i < n; i++ {
			nested = map[string]any{"a": nested}
		}
		var slices any = 1
		for i := 0; i < n; i++ {
			slices = []any{slices}
		}
		pr := func(what string, err error) { fmt.Printf("%s %7d %-26s %s\n", who, n, what, class(err)) }
		_, err := json.Marshal(head)
		pr("Marshal linked list", err)
		_, err = json.Marshal(nested)
		pr("Marshal nested maps", err)
		_, err = json.Marshal(slices)
		pr("Marshal nested slices", err)
		var ss S
		var mm M
		for i := 0; i < n; i++ {
			ss = S{ss}
			mm = M{"a": mm}
		}
		_, err = json.Marshal(ss)
		pr("Marshal slice of slices", err)
		_, err = json.Marshal(mm)
		pr("Marshal map of maps", err)
		ss, mm = nil, nil
		_, err = json.MarshalIndent(slices, "", "  ")
		pr("MarshalIndent slices", err)
		_, err = json.MarshalIndent(nested, ">", "  ")
		pr("MarshalIndent maps", err)
		err = json.NewEncoder(&bytes.Buffer{}).Encode(head)
		pr("Encoder.Encode list", err)
		var dhead *DNode
		for i := 0; i < n; i++ {
			dhead = &DNode{dhead, i}
		}
		_, err = json.Marshal(dhead)
		pr("Marshal derived list", err)
		dhead = nil
		// The shallow ones round trip.
		if n == 200 {
			b, _ := json.Marshal(nested)
			var back any
			fmt.Println(who, "round trip", json.Unmarshal(b, &back), len(b))
		}
		// A deep list is taken apart without recursion by the runtime; drop the references here so that the next round does not hold them.
		head, nested, slices = nil, nil, nil
	}
}

func cycle(who string) {
	n := &DNode{V: 1}
	n.Next = n
	_, err := json.Marshal(n)
	fmt.Println(who, "cycle through a derived type:", class(err))
	n.Next = nil // the cycle is broken, so that it is freed
	p := &Node{V: 1}
	p.Next = p
	_, err = json.Marshal(p)
	fmt.Println(who, "cycle through a plain type:", err)
	p.Next = nil
}

func main() {
	cycle("main")
	// The main thread's stack grows up to 1 GiB (the runtime raises RLIMIT_STACK; without ASLR it ends
	// at about 127 MiB): 9999 levels of Rec and a value of 100000 levels fit there or not depending on
	// the compiler, the mode and the layout, and the guard would only answer after most of a GiB. So the
	// main goroutine runs what is the same everywhere: 200 levels, and the scanner's limit of 10000.
	battery("main", []int{200, 100000, 1000000}, []int{200})
	done := make(chan bool)
	go func() {
		cycle("goroutine")
		battery("goroutine", []int{200, 9999, 100000}, []int{200, 100000})
		done <- true
	}()
	<-done
}

// Output:
// main cycle through a derived type: other: json: unsupported value: encountered a cycle via *main.DNode
// main cycle through a plain type: json: unsupported value: encountered a cycle via *main.Node
// main     200 Unmarshal any              ok
// main     200 Unmarshal any (objects)    ok
// main     200 Unmarshal map              ok
// main     200 Unmarshal Rec              ok
// main     200 Unmarshal Rec2             ok
// main     200 Unmarshal RawMessage       ok
// main     200 Unmarshal Number           other: json: cannot unmarshal array into Go value of type json.Number
// main     200 Unmarshal skipped value    ok
// main     200 Valid                      true
// main     200 Compact                    ok
// main     200 Indent                     ok
// main     200 Decoder.Token              401 tokens, then EOF
// main     200 Decoder.Decode any         ok
// main  100000 Unmarshal any              too deep
// main  100000 Unmarshal any (objects)    too deep
// main  100000 Unmarshal map              too deep
// main  100000 Unmarshal Rec              too deep
// main  100000 Unmarshal Rec2             too deep
// main  100000 Unmarshal RawMessage       too deep
// main  100000 Unmarshal Number           too deep
// main  100000 Unmarshal skipped value    too deep
// main  100000 Valid                      false
// main  100000 Compact                    too deep
// main  100000 Indent                     too deep
// main  100000 Decoder.Token              200001 tokens, then EOF
// main  100000 Decoder.Decode any         too deep
// main 1000000 Unmarshal any              too deep
// main 1000000 Unmarshal any (objects)    too deep
// main 1000000 Unmarshal Rec              too deep
// main 1000000 Unmarshal RawMessage       too deep
// main 1000000 Valid                      false
// main     200 Marshal linked list        ok
// main     200 Marshal nested maps        ok
// main     200 Marshal nested slices      ok
// main     200 Marshal slice of slices    ok
// main     200 Marshal map of maps        ok
// main     200 MarshalIndent slices       ok
// main     200 MarshalIndent maps         ok
// main     200 Encoder.Encode list        ok
// main     200 Marshal derived list       ok
// main round trip <nil> 1201
// goroutine cycle through a derived type: other: json: unsupported value: encountered a cycle via *main.DNode
// goroutine cycle through a plain type: json: unsupported value: encountered a cycle via *main.Node
// goroutine     200 Unmarshal any              ok
// goroutine     200 Unmarshal any (objects)    ok
// goroutine     200 Unmarshal map              ok
// goroutine     200 Unmarshal Rec              ok
// goroutine     200 Unmarshal Rec2             ok
// goroutine     200 Unmarshal RawMessage       ok
// goroutine     200 Unmarshal Number           other: json: cannot unmarshal array into Go value of type json.Number
// goroutine     200 Unmarshal skipped value    ok
// goroutine     200 Valid                      true
// goroutine     200 Compact                    ok
// goroutine     200 Indent                     ok
// goroutine     200 Decoder.Token              401 tokens, then EOF
// goroutine     200 Decoder.Decode any         ok
// goroutine    9999 Unmarshal Rec              too deep
// goroutine    9999 Unmarshal Rec2             too deep
// goroutine  100000 Unmarshal any              too deep
// goroutine  100000 Unmarshal any (objects)    too deep
// goroutine  100000 Unmarshal map              too deep
// goroutine  100000 Unmarshal Rec              too deep
// goroutine  100000 Unmarshal Rec2             too deep
// goroutine  100000 Unmarshal RawMessage       too deep
// goroutine  100000 Unmarshal Number           too deep
// goroutine  100000 Unmarshal skipped value    too deep
// goroutine  100000 Valid                      false
// goroutine  100000 Compact                    too deep
// goroutine  100000 Indent                     too deep
// goroutine  100000 Decoder.Token              200001 tokens, then EOF
// goroutine  100000 Decoder.Decode any         too deep
// goroutine     200 Marshal linked list        ok
// goroutine     200 Marshal nested maps        ok
// goroutine     200 Marshal nested slices      ok
// goroutine     200 Marshal slice of slices    ok
// goroutine     200 Marshal map of maps        ok
// goroutine     200 MarshalIndent slices       ok
// goroutine     200 MarshalIndent maps         ok
// goroutine     200 Encoder.Encode list        ok
// goroutine     200 Marshal derived list       ok
// goroutine round trip <nil> 1201
// goroutine  100000 Marshal linked list        too deep
// goroutine  100000 Marshal nested maps        too deep
// goroutine  100000 Marshal nested slices      too deep
// goroutine  100000 Marshal slice of slices    too deep
// goroutine  100000 Marshal map of maps        too deep
// goroutine  100000 MarshalIndent slices       too deep
// goroutine  100000 MarshalIndent maps         too deep
// goroutine  100000 Encoder.Encode list        too deep
// goroutine  100000 Marshal derived list       too deep
