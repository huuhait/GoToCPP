//go:build !race

package lower

// raceSlowdown scales the time bounds of the tests that measure speed (see race_on_test.go).
const raceSlowdown = 1
