// phase: 03

package main

import "fmt"

type Acct struct {
	name string
	bal  int
	log  []string
}

func (a *Acct) Deposit(xs ...int) int {
	for _, x := range xs {
		a.bal += x
	}
	a.note("deposit", len(xs))
	return a.bal
}

func (a *Acct) note(what string, n int) {
	a.log = append(a.log, what+string(rune('0'+n)))
}

func (a Acct) Report(prefix string, extra ...string) string {
	s := prefix + a.name
	for _, e := range extra {
		s += "," + e
	}
	return s
}

// A closure that captures the pointer receiver changes the account.
func (a *Acct) Spender(limit int) func(int) bool {
	return func(x int) bool {
		if x > limit || x > a.bal {
			return false
		}
		a.bal -= x
		return true
	}
}

// A closure of a value method captures the copy.
func (a Acct) Snapshot() func() int {
	return func() int { return a.bal }
}

func (a *Acct) Close() (final int) {
	defer func() {
		final = a.bal
		a.bal = 0
	}()
	a.bal += 1
	return -1
}

func two() (int, int) { return 7, 8 }

func names() (string, string, string) { return "p:", "x", "y" }

func main() {
	a := &Acct{name: "a"}
	fmt.Println(a.Deposit(), a.Deposit(10, 20), a.Deposit([]int{5}...), a.Deposit(two()))
	snap := a.Snapshot()
	spend := a.Spender(15)
	fmt.Println(spend(10), spend(20), spend(14), a.bal, snap())
	fmt.Println(a.Report("r:"), a.Report("r:", "1", "2"), a.Report(names()))
	var v Acct
	v.name = "v"
	fmt.Println(v.Deposit(1), v.Report("v:"))
	fmt.Println(a.Close(), a.bal, len(a.log))
	defer func() { fmt.Println("deferred", a.Deposit(3)) }()
	defer a.Deposit(100, 200)
}
