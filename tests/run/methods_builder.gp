// phase: 03

// The builder pattern: pointer methods that return their receiver, chained; value methods on
// the result; method values stored in a map and called later (the receiver bound when the value
// was made); a method expression applied to several receivers.
package main

import "fmt"

type Query struct {
	table  string
	wheres []string
	limit  int
}

func From(t string) *Query { return &Query{table: t, limit: -1} }

func (q *Query) Where(c string) *Query {
	q.wheres = append(q.wheres, c)
	return q
}

func (q *Query) Limit(n int) *Query {
	q.limit = n
	return q
}

func (q Query) String() string {
	s := "SELECT * FROM " + q.table
	for i, w := range q.wheres {
		if i == 0 {
			s += " WHERE " + w
		} else {
			s += " AND " + w
		}
	}
	if q.limit >= 0 {
		s += fmt.Sprint(" LIMIT ", q.limit)
	}
	return s
}

type Acc struct{ total int }

func (a *Acc) Add(n int) { a.total += n }
func (a Acc) Get() int   { return a.total }

func main() {
	q := From("users").Where("age > 18").Where("name != ''").Limit(10)
	fmt.Println(q)
	fmt.Println(From("t").String(), *From("x").Limit(0))

	var a Acc
	ops := map[string]func(int){"add": a.Add}
	get := a.Get // a copy of a now: 0
	ops["add"](5)
	ops["add"](7)
	fmt.Println(a.total, get(), a.Get())

	str := Query.String
	for _, x := range []Query{{table: "a", limit: -1}, {table: "b", limit: 3}} {
		fmt.Println(str(x))
	}
	addTo := (*Acc).Add
	addTo(&a, 100)
	fmt.Println(a.Get())
}

// Output:
// SELECT * FROM users WHERE age > 18 AND name != '' LIMIT 10
// SELECT * FROM t SELECT * FROM x LIMIT 0
// 12 0 12
// SELECT * FROM a
// SELECT * FROM b LIMIT 3
// 112
