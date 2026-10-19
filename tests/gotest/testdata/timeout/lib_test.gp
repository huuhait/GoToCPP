package lib

import (
	"testing"
)

func TestHang(t *testing.T) {
	select {}
}
