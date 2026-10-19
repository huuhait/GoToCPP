// phase: 07

// A type set with a method: only defined types with the method may be used; operators and methods both work.
package main

import "fmt"

type Money interface {
	~int64 | ~int
	Cents() int64
	fmt.Stringer
}

type USD int64
type EUR int

func (u USD) Cents() int64   { return int64(u) }
func (u USD) String() string { return fmt.Sprintf("$%d.%02d", u/100, u%100) }
func (e EUR) Cents() int64   { return int64(e) * 100 / 110 }
func (e EUR) String() string { return fmt.Sprintf("€%d", int(e)) }

func Sum[M Money](ms ...M) (M, int64) {
	var total M
	var cents int64
	for _, m := range ms {
		total += m
		cents += m.Cents()
	}
	return total, cents
}

func Fmt[M Money](m M) string { return m.String() + "/" + fmt.Sprint(m.Cents()) }

func Scale[M Money](m M, k int) M {
	for range k - 1 {
		m += m
	}
	return m
}

func Biggest[M Money](ms []M) (best M) {
	for i, m := range ms {
		if i == 0 || m > best {
			best = m
		}
	}
	return
}

func main() {
	fmt.Println(Sum(USD(150), USD(275)))
	fmt.Println(Sum(EUR(110), EUR(220), EUR(1)))
	fmt.Println(Fmt(USD(1999)), Fmt(EUR(22)), Scale(USD(5), 4), Scale(EUR(3), 3))
	fmt.Println(Biggest([]USD{5, 900, 20}), Biggest([]EUR{}), Biggest([]EUR{4, 2}))
	fmt.Printf("%v %s %d %T\n", USD(5), EUR(7), USD(123), Scale(USD(1), 2))
}

// Output:
// $4.25 425
// €331 300
// $19.99/1999 €22/20 $0.40 €12
// $9.00 €0 €4
// $0.05 €7 123 main.USD
