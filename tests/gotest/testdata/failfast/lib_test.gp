package lib

import (
	"testing"
)

func TestA(t *testing.T) { t.Error("first failure") }
func TestB(t *testing.T) { t.Log("not run with -failfast") }
func TestC(t *testing.T) { t.Error("not run either") }
