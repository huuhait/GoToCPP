package lib

import (
	"testing"
)

func TestBefore(t *testing.T) {}

func TestPanics(t *testing.T) {
	t.Log("about to panic")
	panic("boom")
}

func TestNeverRuns(t *testing.T) {}
