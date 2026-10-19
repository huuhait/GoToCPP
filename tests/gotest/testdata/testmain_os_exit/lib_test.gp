package lib

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	fmt.Println("setup", len(os.Args) > 0)
	os.Exit(m.Run())
}

func TestA(t *testing.T) { t.Log("a") }

func TestB(t *testing.T) { t.Error("b fails") }
