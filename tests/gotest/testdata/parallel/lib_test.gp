package lib

import (
	"testing"
)

func TestPar(t *testing.T) {
	t.Run("a", func(t *testing.T) {
		t.Parallel()
		t.Log("a ran")
	})
	t.Run("b", func(t *testing.T) {
		t.Log("b ran first, it is not parallel")
	})
	t.Log("parent body done")
}

func TestSeqAfter(t *testing.T) {
	t.Log("sequential")
}
