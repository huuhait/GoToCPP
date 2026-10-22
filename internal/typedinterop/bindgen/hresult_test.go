package bindgen

import (
	"strings"
	"testing"
)

// The view of a header writes the typedef HRESULT as the result error (ADR-137), the way the call
// of the function is typed in a program; the other result types stay what they are.

func hresultView(t *testing.T) string {
	t.Helper()
	res := generate(t, "hresult.h", nil)
	checkGP(t, "hresult.h", res)
	return string(res.Source)
}

func TestHResultResultIsError(t *testing.T) {
	view := hresultView(t)
	if !strings.Contains(view, "func create_device(flags C.int) error") {
		t.Errorf("the function that returns HRESULT does not return error:\n%s", view)
	}
}

func TestHResultCallbackResultIsError(t *testing.T) {
	view := hresultView(t)
	if !strings.Contains(view, "func(C.int) error") {
		t.Errorf("the callback that returns HRESULT does not return error:\n%s", view)
	}
}

func TestHResultLeavesOtherResults(t *testing.T) {
	view := hresultView(t)
	for _, fn := range []string{"my_call", "window_proc", "open_file", "tick_count"} {
		for _, line := range strings.Split(view, "\n") {
			if strings.HasPrefix(line, "func "+fn+"(") && strings.HasSuffix(line, "error") {
				t.Errorf("%s returns error: %s", fn, line)
			}
		}
	}
}
