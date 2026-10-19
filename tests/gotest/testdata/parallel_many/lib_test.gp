package lib

import (
	"testing"
)

func TestMany(t *testing.T) {
	done := make(chan int, 20)
	for i := 0; i < 20; i++ {
		t.Run("n", func(t *testing.T) {
			t.Parallel()
			done <- 1
		})
	}
}
