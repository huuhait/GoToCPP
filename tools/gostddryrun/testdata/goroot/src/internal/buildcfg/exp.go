// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package buildcfg

import "internal/goexperiment"

// ExperimentFlags is the baseline and the flags of one configuration.
type ExperimentFlags struct {
	goexperiment.Flags
	baseline goexperiment.Flags
}

// ParseGOEXPERIMENT parses a (GOOS, GOARCH, GOEXPERIMENT) tuple.
func ParseGOEXPERIMENT(goos, goarch, goexp string) (*ExperimentFlags, error) {
	var regabiSupported, regabiAlwaysOn bool
	switch goarch {
	case "amd64", "arm64", "loong64", "ppc64le", "ppc64", "riscv64":
		regabiAlwaysOn = true
		regabiSupported = true
	}

	dwarf5Supported := (goos != "darwin" && goos != "ios" && goos != "aix")

	baseline := goexperiment.Flags{
		RegabiWrappers:  regabiSupported,
		RegabiArgs:      regabiSupported,
		AliasTypeParams: true,
		SwissMap:        true,
		SyncHashTrieMap: true,
		Dwarf5:          dwarf5Supported,
	}

	flags := &ExperimentFlags{
		Flags:    baseline,
		baseline: baseline,
	}
	if regabiAlwaysOn {
		flags.RegabiWrappers = true
		flags.RegabiArgs = true
	}
	if !regabiSupported {
		flags.RegabiWrappers = false
		flags.RegabiArgs = false
	}
	return flags, nil
}
