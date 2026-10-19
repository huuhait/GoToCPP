package lib

import (
	"testing"
)

func TestWorkers(t *testing.T) {
	done := make(chan bool)
	for i := 0; i < 3; i++ {
		go func() {
			t.Log("worker log")
			done <- true
		}()
	}
	for i := 0; i < 3; i++ {
		<-done
	}
}
