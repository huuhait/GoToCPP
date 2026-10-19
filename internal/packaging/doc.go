// Package packaging has no code: its tests check how gocpp is installed and released (step 15.13,
// docs/guides/packaging.md): that go install of a tagged version works (module zip rules, the
// embedded runtime, the version that the binary reports), that .goreleaser.yaml builds what the
// plan promises, and that flake.nix keeps up with go.mod.
package packaging
