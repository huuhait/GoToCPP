// phase: 09
// A *regexp.Regexp is safe for concurrent use: 12 goroutines run Match, Find*, Replace*, Split and FindAll on a few shared Regexps (the machines
// come from a sync.Pool, the backtracker and the one-pass matcher have their own pools), on inputs that differ per goroutine, and each goroutine's
// hash of the results must equal the hash of the same work done in order. Longest is set before the Regexp is shared, as the documentation
// asks. Regexp.Copy gives each goroutine a Regexp of its own for the same inputs. The expected output comes from real Go 1.25
// (GOCPP_RUN_VS_GO=1 checks it again); `gocpp run -race` (make ci-race) must report nothing.

package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

type rngT struct{ s uint64 }

func (r *rngT) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
func (r *rngT) intn(n int) int { return int(r.next()>>1) % n }

type hasher struct{ h uint64 }

func (x *hasher) str(s string) {
	for i := 0; i < len(s); i++ {
		x.h ^= uint64(s[i])
		x.h *= 1099511628211
	}
	x.h ^= 0xff
	x.h *= 1099511628211
}

var pool = []string{"a", "b", "c", "ab", "é", "世", "\n", " ", "1", "_", "foo", "bar@baz.com", "x=1", "2024-05-17", "\xff", "AAA"}

func input(r *rngT) string {
	var b strings.Builder
	for n := 5 + r.intn(60); n > 0; n-- {
		b.WriteString(pool[r.intn(len(pool))])
	}
	return b.String()
}

const nworkers = 12
const iterations = 40

func work(res []*regexp.Regexp, id int) uint64 {
	r := &rngT{uint64(id) * 7919}
	h := &hasher{14695981039346656037}
	for it := 0; it < iterations; it++ {
		s := input(r)
		for _, re := range res {
			if re.MatchString(s) {
				h.str("y")
			}
			h.str(fmt.Sprint(re.FindStringSubmatchIndex(s)))
			h.str(fmt.Sprint(re.FindAllStringIndex(s, -1)))
			h.str(re.ReplaceAllString(s, "<$0>"))
			h.str(fmt.Sprintf("%q", re.Split(s, 4)))
			h.str(fmt.Sprint(re.MatchReader(strings.NewReader(s))))
		}
	}
	return h.h
}

func main() {
	pats := []string{`(\w+)@(\w+)\.com`, `^(\d+)-(\d+)-(\d+)`, `a+b?|c`, `(?i)foo|bar`, `x=(\d)`, `[^\n]+\n`, `(a|ab)(c|bcd)?`, `\p{L}+`}
	var shared []*regexp.Regexp
	for _, p := range pats {
		shared = append(shared, regexp.MustCompile(p))
	}
	longest := regexp.MustCompile(`(a|ab)(c|bcd)?`)
	longest.Longest()
	shared = append(shared, longest)

	want := make([]uint64, nworkers)
	for id := range want {
		want[id] = work(shared, id)
	}

	got := make([]uint64, nworkers)
	var wg sync.WaitGroup
	for id := 0; id < nworkers; id++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[id] = work(shared, id)
		}()
	}
	wg.Wait()

	copies := make([]uint64, nworkers)
	for id := 0; id < nworkers; id++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mine := make([]*regexp.Regexp, len(shared))
			for i, re := range shared {
				mine[i] = re.Copy()
			}
			copies[id] = work(mine, id)
		}()
	}
	wg.Wait()

	bad := 0
	for id := range want {
		if got[id] != want[id] || copies[id] != want[id] {
			bad++
			fmt.Println("worker", id, "differs")
		}
	}
	var total uint64
	for _, w := range want {
		total ^= w
	}
	fmt.Println("workers:", nworkers, "differ:", bad)
	fmt.Printf("hash %016x\n", total)
}

// Output:
// workers: 12 differ: 0
// hash a996f125cc99a4db
