// phase: 09
// divergence: D24

// The stack guard of encoding/json (step 9.11 review) at the places that json_deep_input does not reach: each of the checks of the encoder and the
// decoder has an input here that only it can stop, so that taking one away ends the program with "fatal error: stack overflow" instead of the
// error. interfaceEncoder: a struct that holds an interface that holds a struct (no pointer, slice or map in the cycle). textMarshalerEncoder: a
// MarshalText that calls Marshal. addrMarshalerEncoder and addrTextMarshalerEncoder: the same with pointer receivers, reached through a pointer
// (so ptrEncoder is in the cycle, and its check would stop the recursion too: these two checks cannot be proved by an input, only exercised).
// valueInterface (the decoder into any): the scanner's limit of 10000 comes before the stack's in a build with a large stack, so the stack is made
// small by eating it with a recursion, up to the point where a decode of 2000 levels no longer fits; the decode of 9000 levels there must fail
// cleanly. (Where the stack cannot be eaten this way, the AddressSanitizer's fake stacks, the decode just succeeds and nothing is proved.)

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type SI struct{ X any }

type TM struct{ n int }

func (t TM) MarshalText() ([]byte, error) {
	if t.n == 0 {
		return []byte("end"), nil
	}
	if _, err := json.Marshal(TM{t.n - 1}); err != nil {
		return nil, err
	}
	return []byte("text"), nil
}

type PT struct{ n int }

func (p *PT) MarshalText() ([]byte, error) {
	if p.n == 0 {
		return []byte("end"), nil
	}
	if _, err := json.Marshal(&struct{ V PT }{PT{p.n - 1}}); err != nil {
		return nil, err
	}
	return []byte("text"), nil
}

type PM struct{ n int }

func (p *PM) MarshalJSON() ([]byte, error) {
	if p.n == 0 {
		return []byte(`"end"`), nil
	}
	if _, err := json.Marshal(&struct{ V PM }{PM{p.n - 1}}); err != nil {
		return nil, err
	}
	return []byte(`"json"`), nil
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

// eat uses about 16 KB of stack for each level it recurses, then calls f.
func eat(n int, f func()) byte {
	var pad [16384]byte
	pad[n%len(pad)] = byte(n)
	var r byte
	if n > 0 {
		r = eat(n-1, f)
	} else {
		f()
	}
	return r + pad[(n*7+1)%len(pad)]
}

// decodeAfterEating decodes doc into an any on a new goroutine, after k levels of eat.
func decodeAfterEating(k int, doc string) error {
	done := make(chan error)
	go func() {
		var err error
		eat(k, func() {
			var a any
			err = json.Unmarshal([]byte(doc), &a)
		})
		done <- err
	}()
	return <-done
}

func main() {
	for _, n := range []int{200, 1000000} {
		var v any = 1
		for i := 0; i < min(n, 100000); i++ {
			v = SI{v}
		}
		_, err := json.Marshal(v)
		fmt.Printf("%7d %-34s %s\n", n, "Marshal struct{any} nesting", class(err))
		v = nil
		_, err = json.Marshal(TM{n})
		fmt.Printf("%7d %-34s %s\n", n, "Marshal MarshalText calling Marshal", class(err))
		_, err = json.Marshal(&struct{ V PT }{PT{n}})
		fmt.Printf("%7d %-34s %s\n", n, "Marshal pointer MarshalText", class(err))
		_, err = json.Marshal(&struct{ V PM }{PM{n}})
		fmt.Printf("%7d %-34s %s\n", n, "Marshal pointer MarshalJSON", class(err))
	}

	// The deepest of 8000, 4000 and 2000 levels that a fresh stack holds in this build (about 9100 at most, with g++ -O0).
	big := strings.Repeat("[", 9999) + "1" + strings.Repeat("]", 9999)
	probe := ""
	for _, d := range []int{8000, 4000, 2000} {
		probe = strings.Repeat("[", d) + "1" + strings.Repeat("]", d)
		if decodeAfterEating(0, probe) == nil {
			break
		}
	}
	// The most that can be eaten (440 levels, 7.2 MiB of the 8 MiB) leaves the runtime's reserve and more. Where the probe still fits then (the stack
	// cannot be eaten: the fake stacks of the AddressSanitizer), there is no boundary and the search is skipped.
	lo, hi := 0, 440
	if decodeAfterEating(hi, probe) != nil {
		for hi-lo > 1 {
			mid := (lo + hi) / 2
			if decodeAfterEating(mid, probe) == nil {
				lo = mid
			} else {
				hi = mid
			}
		}
	}
	// At the boundary the probe fits and a few more levels of eating would not hold it: the decode of 9999 levels must run out and say so.
	err := decodeAfterEating(lo, big)
	fmt.Println("decode 9999 levels into any with little stack left: clean:", err == nil || class(err) == "too deep")
}

// Output:
//     200 Marshal struct{any} nesting        ok
//     200 Marshal MarshalText calling Marshal ok
//     200 Marshal pointer MarshalText        ok
//     200 Marshal pointer MarshalJSON        ok
// 1000000 Marshal struct{any} nesting        too deep
// 1000000 Marshal MarshalText calling Marshal too deep
// 1000000 Marshal pointer MarshalText        too deep
// 1000000 Marshal pointer MarshalJSON        too deep
// decode 9999 levels into any with little stack left: clean: true
