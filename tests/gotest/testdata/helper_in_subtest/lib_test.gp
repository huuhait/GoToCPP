package lib

import "testing"

func h(t *testing.T) {
	t.Helper()
	t.Log("in h")
}

func TestSub(t *testing.T) {
	t.Run("all helpers", func(t *testing.T) {
		t.Helper()
		h(t)
	})
	t.Run("one level", func(t *testing.T) {
		h(t)
		t.Log("after h")
	})
	t.Run("nested", func(t *testing.T) {
		t.Helper()
		t.Run("inner", func(t *testing.T) {
			t.Helper()
			t.Error("inner error")
		})
	})
}

func TestTopHelper(t *testing.T) {
	t.Helper()
	t.Log("test is a helper")
}
