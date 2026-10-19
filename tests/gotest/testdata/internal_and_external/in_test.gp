package lib

import (
	"testing"
)

func TestInternal(t *testing.T) {
	if secret() != 7 {
		t.Fatal("bad")
	}
}
