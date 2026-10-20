package bg

// The caller installs Hook before StartWorker and does not change it while the worker runs.
var Hook func()

func StartWorker() {
	go func() { Hook() }()
}
