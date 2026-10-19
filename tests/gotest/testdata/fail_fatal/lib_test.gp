package lib

import (
	"testing"
)

func TestStops(t *testing.T) {
	t.Log("before")
	t.Fatalf("stop here: %v", "now")
	t.Log("never printed")
}

func TestAfter(t *testing.T) {
	t.Log("still runs")
}
