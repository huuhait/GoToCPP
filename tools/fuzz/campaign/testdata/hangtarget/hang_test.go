// Package hangtarget is a fuzz target that hangs on an input that starts with "hang": the campaign
// runner's test runs it (tools/fuzz/campaign/main_test.go, TestHangIsReported). It is in testdata
// so that ./... does not run it.
package hangtarget

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/tools/fuzz/fuzzkit"
)

func spinForever() {
	for {
	}
}

func FuzzHang(f *testing.F) {
	f.Add("han")
	f.Fuzz(func(t *testing.T, s string) {
		fuzzkit.Guard("FuzzHang", func() {
			if strings.HasPrefix(s, "hang") {
				spinForever()
			}
		})
	})
}
