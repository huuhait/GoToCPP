package lib

import "testing"

func h(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { t.Helper(); t.Log("cleanup in helper") })
}

func TestA(t *testing.T) { h(t) }
