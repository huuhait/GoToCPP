package lib

import (
	"testing"
)

func TestSlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Log("full run")
}

func TestVerbose(t *testing.T) {
	t.Logf("verbose=%v", testing.Verbose())
}
