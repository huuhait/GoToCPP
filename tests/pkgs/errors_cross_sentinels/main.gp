// phase: 06

// Sentinel errors of two packages with the same text: == and errors.Is tell them apart, also through a wrapper type that only the cache package knows.

package main

import (
	"errors"
	"fmt"

	"example.com/sent/cache"
	"example.com/sent/store"
)

func describe(err error) string {
	switch {
	case err == nil:
		return "ok"
	case err == store.ErrNotFound:
		return "store.ErrNotFound itself"
	case errors.Is(err, store.ErrNotFound):
		return "wraps store.ErrNotFound"
	case errors.Is(err, store.ErrClosed):
		return "wraps store.ErrClosed"
	case errors.Is(err, cache.ErrNotFound):
		return "wraps cache.ErrNotFound"
	}
	return "unknown: " + err.Error()
}

func main() {
	s := store.New()
	_, err := s.Get("a")
	fmt.Println(describe(err))
	_, err = s.Get("zz")
	fmt.Println(describe(err))
	_, err = cache.Lookup(s, "zz")
	fmt.Println(describe(err), "|", err.Error())
	fmt.Println(err == store.ErrNotFound, errors.Unwrap(err) == store.ErrNotFound)
	fmt.Println(store.ErrNotFound == cache.ErrNotFound, store.ErrNotFound.Error() == cache.ErrNotFound.Error())
	fmt.Println(describe(cache.Own("")), describe(cache.Own("x")))
	s.Close()
	_, err = cache.Lookup(s, "a")
	fmt.Println(describe(err))
	counts := map[error]int{}
	for _, e := range []error{store.ErrNotFound, cache.ErrNotFound, store.ErrNotFound, store.ErrClosed} {
		counts[e]++
	}
	fmt.Println(len(counts), counts[store.ErrNotFound], counts[cache.ErrNotFound])
}

// Output:
// ok
// store.ErrNotFound itself
// wraps store.ErrNotFound | cache miss for zz: not found
// false true
// false true
// wraps cache.ErrNotFound ok
// wraps store.ErrClosed
// 3 2 1
