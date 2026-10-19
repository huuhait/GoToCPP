package lib

import (
	"testing"
)

func TestBad(t *testing.T) {
	t.Log("shown because it fails")
	t.Fail()
}
