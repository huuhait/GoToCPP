package lib

import (
	"testing"
)

func TestSkipf(t *testing.T) {
	t.Skipf("skipping %s", "this")
}

func TestSkipNow(t *testing.T) {
	t.SkipNow()
}

func TestSkipAfterFail(t *testing.T) {
	t.Fail()
	t.Skip("skipped but failed")
}

func TestSkipped(t *testing.T) {
	t.Skip()
	if !t.Skipped() {
		t.Error("unreachable")
	}
}
