// phase: 05

package util

import "fmt"

type secret struct{ n int }

type Public struct{ N int }

macro hidden() {}

macro Take(f lazy secret) int { return f().n }

macro Show(f lazy fmt.Stringer) string { return f().String() }

macro Fine(f lazy Public) int { return f().N }

macro Plain(n int) int { return n }

func Use() int {
	#hidden()
	return #Take(secret{1}) + #Fine(Public{2})
}
