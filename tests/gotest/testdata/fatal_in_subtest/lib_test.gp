package lib

import (
	"testing"
)

func TestParent(t *testing.T) {
	ok := t.Run("child", func(t *testing.T) {
		t.Fatal("child failed")
	})
	t.Logf("child ok=%v", ok)
	t.Run("sibling", func(t *testing.T) {
		t.Log("sibling runs")
	})
}
