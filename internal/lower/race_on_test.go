//go:build race

package lower

// raceSlowdown scales the time bounds of the tests that measure speed: the race detector slows the
// lowering down by an order of magnitude.
const raceSlowdown = 10
