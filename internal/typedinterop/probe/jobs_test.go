package probe

import (
	"runtime"
	"testing"
)

func TestJobsDefaultOfABuildLeavesHalfTheCores(t *testing.T) {
	t.Setenv(EnvJobs, "")
	SetInteractive(false)
	want := max(1, min(8, runtime.NumCPU()/2))
	if got := Jobs(); got != want {
		t.Errorf("Jobs() = %d, want %d", got, want)
	}
}

func TestJobsDefaultOfALanguageServerUsesAllCoresUpToEight(t *testing.T) {
	t.Setenv(EnvJobs, "")
	SetInteractive(true)
	defer SetInteractive(false)
	want := max(1, min(8, runtime.NumCPU()))
	if got := Jobs(); got != want {
		t.Errorf("Jobs() = %d, want %d", got, want)
	}
}

func TestJobsEnvironmentOverridesTheLanguageServerDefault(t *testing.T) {
	t.Setenv(EnvJobs, "3")
	SetInteractive(true)
	defer SetInteractive(false)
	if got := Jobs(); got != 3 {
		t.Errorf("Jobs() = %d, want 3", got)
	}
}
