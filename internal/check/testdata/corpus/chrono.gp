// std::chrono durations as dotted type names: variables, parameters,
// results, struct fields and slices.
package main

import (
	"fmt"

	"<chrono>"
	"<thread>"
)

type Timer struct {
	started std.chrono.steady_clock.time_point
	budget  std.chrono.milliseconds
	laps    []std.chrono.milliseconds
}

func wait(d std.chrono.milliseconds) std.chrono.milliseconds {
	std.this_thread.sleep_for(d)
	return d
}

func (t *Timer) lap() {
	now := std.chrono.steady_clock.now()
	t.laps = append(t.laps, std.chrono.duration_cast[std.chrono.milliseconds](now-t.started))
}

func main() {
	t := &Timer{started: std.chrono.steady_clock.now(), budget: std.chrono.milliseconds(5)}
	var d std.chrono.milliseconds = wait(std.chrono.milliseconds(1))
	t.lap()
	total := int64(d.count())
	for _, l := range t.laps {
		total += int64(l.count())
	}
	var secs std.chrono.seconds = std.chrono.seconds(2)
	fmt.Println(total >= 1, int64(secs.count()), len(t.laps), t.budget.count())
}
