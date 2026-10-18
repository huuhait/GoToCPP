package util

import "example.com/fmtx/model"

type Wrapper struct {
	model.Account
	Extra model.Money
	Pts   []any
}

func Wrap(a *model.Account) Wrapper {
	return Wrapper{*a, 9, []any{model.NewPoint(1, 2), model.Money(3)}}
}
