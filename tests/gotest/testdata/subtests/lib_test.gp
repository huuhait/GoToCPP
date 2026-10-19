package lib

import (
	"testing"
)

func TestTable(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b int
		want int
	}{
		{"small", 1, 2, 3},
		{"zero", 0, 0, 0},
		{"with space", 5, 5, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Add(tc.a, tc.b); got != tc.want {
				t.Errorf("got %d want %d", got, tc.want)
			}
			t.Log("ran", t.Name())
		})
	}
}

func TestNested(t *testing.T) {
	t.Run("outer", func(t *testing.T) {
		t.Run("inner", func(t *testing.T) {
			t.Log("deep")
		})
		t.Log("after inner")
	})
}
