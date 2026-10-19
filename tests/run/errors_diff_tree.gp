// phase: 09
// errors against Go on big error trees: Is, As, Unwrap and Join over generated trees of wrapped and joined errors with custom Is and As methods, comparable and uncomparable targets, nil entries, pointers and values, interface targets; every result hashed and the first ones printed.

package main

import (
	"errors"
	"fmt"
)

type vErr struct{ n int }

func (e vErr) Error() string { return fmt.Sprint("v", e.n) }

type pErr struct{ n int }

func (e *pErr) Error() string { return fmt.Sprint("p", e.n) }

type sliceErr struct{ xs []int }

func (e sliceErr) Error() string { return fmt.Sprint("slice", e.xs) }

type wrap1 struct {
	msg string
	err error
}

func (w *wrap1) Error() string { return w.msg + ": " + fmt.Sprint(w.err) }
func (w *wrap1) Unwrap() error { return w.err }

type wrapN struct{ errs []error }

func (w wrapN) Error() string   { return fmt.Sprint("multi", len(w.errs)) }
func (w wrapN) Unwrap() []error { return w.errs }

type isErr struct{ k int }

func (e isErr) Error() string { return fmt.Sprint("is", e.k) }
func (e isErr) Is(t error) bool {
	v, ok := t.(vErr)
	return ok && v.n%5 == e.k%5
}

type asErr struct{ k int }

func (e asErr) Error() string { return fmt.Sprint("as", e.k) }
func (e asErr) As(t any) bool {
	switch p := t.(type) {
	case *vErr:
		*p = vErr{e.k * 100}
		return true
	case **pErr:
		*p = &pErr{e.k}
		return e.k%2 == 0
	}
	return false
}

type stringer interface{ Error() string }

var sentinels = []error{errors.New("s0"), errors.New("s1"), errors.New("s2"), vErr{1}, vErr{2}}

type gen struct{ s uint64 }

func (g *gen) next() int {
	g.s += 0x9E3779B97F4A7C15
	z := g.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return int((z ^ (z >> 31)) >> 33)
}

func (g *gen) tree(depth int) error {
	k := g.next() % 12
	if depth == 0 {
		k %= 6
	}
	switch k {
	case 0:
		return sentinels[g.next()%len(sentinels)]
	case 1:
		return vErr{g.next() % 10}
	case 2:
		return &pErr{g.next() % 10}
	case 3:
		return sliceErr{[]int{g.next() % 3}}
	case 4:
		return isErr{g.next() % 10}
	case 5:
		return asErr{g.next() % 10}
	case 6, 7:
		return &wrap1{"w", g.tree(depth - 1)}
	case 8:
		return fmt.Errorf("f%d: %w", k, g.tree(depth-1))
	case 9:
		n := g.next() % 4
		var es []error
		for i := 0; i < n; i++ {
			if g.next()%5 == 0 {
				es = append(es, nil)
			} else {
				es = append(es, g.tree(depth-1))
			}
		}
		return wrapN{es}
	case 10:
		return errors.Join(g.tree(depth-1), g.tree(depth-1), nil)
	default:
		return fmt.Errorf("two: %w and %w", g.tree(depth-1), g.tree(depth-1))
	}
}

func main() {
	g := &gen{1}
	var h uint64 = 14695981039346656037
	mix := func(v uint64) { h ^= v; h *= 1099511628211 }
	counts := map[string]int{}
	for i := 0; i < 3000; i++ {
		err := g.tree(5)
		for ti, t := range sentinels {
			r := errors.Is(err, t)
			if r {
				mix(uint64(ti*2 + 1))
				counts[fmt.Sprint("is", ti)]++
			} else {
				mix(uint64(ti * 2))
			}
		}
		var v vErr
		var p *pErr
		var s sliceErr
		var ia isErr
		var st stringer
		var e error
		var w *wrap1
		r1, r2, r3, r4, r5, r6, r7 := errors.As(err, &v), errors.As(err, &p), errors.As(err, &s), errors.As(err, &ia), errors.As(err, &st), errors.As(err, &e), errors.As(err, &w)
		for _, b := range []bool{r1, r2, r3, r4, r5, r6, r7} {
			if b {
				mix(1)
			} else {
				mix(0)
			}
		}
		if r1 {
			mix(uint64(v.n))
		}
		if r2 {
			mix(uint64(p.n))
		}
		if r3 {
			mix(uint64(s.xs[0]))
		}
		if r4 {
			mix(uint64(ia.k))
		}
		if r5 {
			mix(uint64(len(st.Error())))
		}
		if r6 && e != nil {
			mix(uint64(len(e.Error())))
		}
		if r7 {
			mix(uint64(len(w.msg)))
		}
		u := errors.Unwrap(err)
		if u != nil {
			mix(uint64(len(u.Error())))
		}
		mix(uint64(len(err.Error())))
		if i < 12 {
			fmt.Println(err, "|", errors.Is(err, sentinels[3]), r1, r2, r3, r4, r5, r6, r7, u != nil)
		}
	}
	fmt.Printf("hash %016x\n", h)
	fmt.Println(counts)

	// Join
	fmt.Println(errors.Join() == nil, errors.Join(nil) == nil, errors.Join(nil, nil) == nil)
	j := errors.Join(errors.New("a"), nil, errors.New("b\nc"))
	fmt.Printf("%q %v\n", j.Error(), errors.Unwrap(j) == nil)
	if m, ok := j.(interface{ Unwrap() []error }); ok {
		fmt.Println(len(m.Unwrap()))
	}
	j1 := errors.Join(sentinels[0])
	fmt.Println(j1 == sentinels[0], errors.Is(j1, sentinels[0]), j1)

	// uncomparable targets and values never panic in Is
	u1, u2 := sliceErr{[]int{1}}, sliceErr{[]int{1}}
	fmt.Println(errors.Is(u1, u2), errors.Is(&wrap1{"x", u1}, u1), errors.Is(u1, u1), errors.Is(nil, nil), errors.Is(u1, nil), errors.Is(nil, u1))
	fmt.Println(errors.Is(isErr{3}, vErr{3}), errors.Is(isErr{3}, vErr{8}), errors.Is(isErr{3}, vErr{4}))

	// As panics
	for i, target := range []any{nil, 5, (*int)(nil), new(int), new(string), new(*int), new(error), new(vErr), new(fmt.Stringer)} {
		func() {
			defer func() { fmt.Println(i, recover()) }()
			fmt.Println(i, errors.As(vErr{1}, target))
		}()
	}
	fmt.Println(errors.As(nil, new(error)), errors.As(nil, 3))
}

// Output:
// p8 | false false true false false true true false false
// slice[1] | false false false true false true true false false
// s1 | false false false false false true true false false
// p5 | false false true false false true true false false
// slice[0] | false false false true false true true false false
// slice[2] | false false false true false true true false false
// multi3 | false true true true true true true true false
// v1 | true true false false false true true false false
// slice[2] | false false false true false true true false false
// w: multi1 | false false false false false true true true true
// slice[1] | false false false true false true true false false
// v1 | true true false false false true true false false
// hash e81559a5b3e9d049
// map[is0:157 is1:145 is2:157 is3:348 is4:364]
// true true true
// "a\nb\nc" true
// 2
// false true s0
// false false false true false false
// true true false
// 0 errors: target cannot be nil
// 1 errors: target must be a non-nil pointer
// 2 errors: target must be a non-nil pointer
// 3 errors: *target must be interface or implement error
// 4 errors: *target must be interface or implement error
// 5 errors: *target must be interface or implement error
// 6 true
// 6 <nil>
// 7 true
// 7 <nil>
// 8 false
// 8 <nil>
// false false
