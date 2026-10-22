// Package devenv holds the tests of scripts/in-devenv, which runs a command in the Nix dev shell
// from a cached environment file instead of `nix develop` (that copies the whole source tree
// into /nix/store at every call when the git tree is dirty). The tests use a fake `nix`.
package devenv
