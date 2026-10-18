package model

import "fmt"

// Money has a String method; Account has exported and unexported fields of types of both packages.
type Money int

func (m Money) String() string { return fmt.Sprintf("$%d", int(m)) }

type Tags []string

type Index map[string]Money

type Account struct {
	Owner   string
	Balance Money
	hidden  Money
	Tags    Tags
	ByName  Index
	Next    *Account
	Err     error
}

type Err struct{ Code int }

func (e *Err) Error() string { return fmt.Sprintf("model error %d", e.Code) }

type pt struct{ X, Y int }

func NewPoint(x, y int) any { return pt{x, y} }

func NewAccount(owner string) *Account {
	return &Account{Owner: owner, Balance: 5, hidden: 6, Tags: Tags{"a", "b"}, ByName: Index{"k": 7}, Err: &Err{3}}
}
