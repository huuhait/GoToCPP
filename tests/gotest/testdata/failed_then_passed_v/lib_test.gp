package lib

import (
	"testing"
)

func TestFirst(t *testing.T) {
	t.Run("sub1", func(t *testing.T) { t.Error("e1") })
	t.Run("sub2", func(t *testing.T) { t.Log("ok2") })
}

func TestSecond(t *testing.T) {}
