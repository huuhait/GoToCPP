package lib

import (
	"testing"
)

func TestLogs(t *testing.T) {
	t.Log("multi\nline\nmessage")
	t.Log("a", 1, true, 2.5)
	t.Logf("no newline")
	t.Logf("with newline\n")
	t.Log()
}
