// phase: 05

package main

import "fmt"

type Account struct {
	Balance int
	History []string
}

func (a *Account) Withdraw(n int) (err string) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Sprint("failed: ", r)
			a.History = append(a.History, "failed")
		}
	}()
	if n > a.Balance {
		panic(fmt.Sprint("insufficient funds: ", n, " > ", a.Balance))
	}
	a.Balance -= n
	a.History = append(a.History, fmt.Sprint("withdrew ", n))
	return "ok"
}

func (a Account) Report() string {
	defer func() { recover() }()
	return fmt.Sprint(a.Balance, " ", len(a.History), " ", a.History[5])
}

func main() {
	a := &Account{Balance: 100}
	fmt.Println(a.Withdraw(30))
	fmt.Println(a.Withdraw(100))
	fmt.Println(a.Withdraw(70))
	fmt.Printf("%q\n", a.Report())
	fmt.Println(*a)
	var nilAcc *Account
	func() {
		defer func() { fmt.Println("nil receiver:", recover()) }()
		fmt.Println(nilAcc.Withdraw(1))
	}()
}
