package lib

import (
	"fmt"
	"testing"
)

func TestMain(m *testing.M) {
	fmt.Println("setup")
	code := m.Run()
	fmt.Println("teardown")
	_ = code
}

func TestOne(t *testing.T) { t.Log("one") }
