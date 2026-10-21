package racetest

import (
	"os"
	"testing"
)

func TestExitChildrenQuicklySetsTheOption(t *testing.T) {
	t.Setenv("GORACE", "")
	os.Unsetenv("GORACE")
	ExitChildrenQuickly()
	if got := os.Getenv("GORACE"); got != ChildExitSetting {
		t.Errorf("GORACE = %q, want %q", got, ChildExitSetting)
	}
}

func TestExitChildrenQuicklyKeepsOtherOptions(t *testing.T) {
	t.Setenv("GORACE", "halt_on_error=1")
	ExitChildrenQuickly()
	if got, want := os.Getenv("GORACE"), "halt_on_error=1 "+ChildExitSetting; got != want {
		t.Errorf("GORACE = %q, want %q", got, want)
	}
}
