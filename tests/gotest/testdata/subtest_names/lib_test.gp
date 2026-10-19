package lib

import (
	"testing"
)

func TestNames(t *testing.T) {
	t.Run("same", func(t *testing.T) { t.Log(t.Name()) })
	t.Run("same", func(t *testing.T) { t.Log(t.Name()) })
	t.Run("", func(t *testing.T) { t.Log(t.Name()) })
	t.Run("a b/c", func(t *testing.T) { t.Log(t.Name()) })
}
