// phase: 09
// encoding/json with the standard types that carry their own JSON form: time.Time (zones,
// precision, range errors), time.Duration, time.Month and Weekday, errors, and bytes.Buffer fields.

package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type Event struct {
	At       time.Time            `json:"at"`
	Maybe    *time.Time           `json:"maybe,omitempty"`
	Took     time.Duration        `json:"took"`
	Month    time.Month           `json:"month"`
	Day      time.Weekday         `json:"day"`
	Zero     time.Time            `json:"zero,omitzero"`
	Optional []time.Time          `json:"optional"`
	ByName   map[string]time.Time `json:"byName"`
}

func main() {
	fixed := time.FixedZone("custom", -(3*3600 + 30*60))
	times := []time.Time{
		time.Date(2024, 2, 29, 23, 59, 59, 0, time.UTC),
		time.Date(2024, 2, 29, 23, 59, 59, 123456789, time.UTC),
		time.Date(2024, 2, 29, 23, 59, 59, 120000000, time.UTC),
		time.Date(2024, 2, 29, 12, 0, 0, 0, fixed),
		time.Date(2024, 2, 29, 12, 0, 0, 1000, time.FixedZone("", 5*3600)),
		time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC),
		time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.FixedZone("x", 24*3600)),
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.FixedZone("x", 59)),
		time.Unix(0, 0).UTC(),
		{},
	}
	fmt.Println("== time.Time encode")
	for _, instant := range times {
		encoded, err := json.Marshal(instant)
		fmt.Printf("%s | %s %v\n", instant.Format(time.RFC3339Nano), encoded, err)
	}

	fmt.Println("== time.Time decode")
	for _, input := range []string{
		`"2024-02-29T23:59:59Z"`, `"2024-02-29T23:59:59.5Z"`, `"2024-02-29T23:59:59.123456789012Z"`, `"2024-02-29T23:59:59+05:30"`,
		`"2024-02-29T23:59:59-00:00"`, `"2024-02-29t23:59:59z"`, `"2024-02-29 23:59:59Z"`, `"2024-02-30T00:00:00Z"`, `"2023-02-29T00:00:00Z"`,
		`"2024-13-01T00:00:00Z"`, `"2024-01-01T24:00:00Z"`, `"2024-01-01T23:60:00Z"`, `"2024-01-01T23:59:60Z"`, `"2024-01-01T00:00:00"`,
		`"2024-01-01"`, `""`, `"x"`, `null`, `1`, `[]`, `"2024-01-01T00:00:00+24:00"`, `"2024-01-01T00:00:00+23:59"`, `"0000-01-01T00:00:00Z"`,
		`"2024-01-01T00:00:00,5Z"`, `"2024-01-01T00:00:00.Z"`, `"+2024-01-01T00:00:00Z"`, `"2024-01-01T00:00:00Z "`, `" 2024-01-01T00:00:00Z"`,
	} {
		var instant time.Time
		err := json.Unmarshal([]byte(input), &instant)
		_, offset := instant.Zone()
		fmt.Printf("%-40s => %s offset=%d err=%v\n", input, instant.Format(time.RFC3339Nano), offset, err)
	}

	fmt.Println("== struct")
	moment := time.Date(2020, 5, 17, 8, 30, 0, 500, fixed)
	event := Event{At: moment, Maybe: &moment, Took: 1500 * time.Millisecond, Month: time.March, Day: time.Friday,
		Optional: []time.Time{moment, {}}, ByName: map[string]time.Time{"b": moment, "a": {}}}
	encoded, err := json.Marshal(event)
	fmt.Printf("%s %v\n", encoded, err)
	var back Event
	err = json.Unmarshal(encoded, &back)
	reencoded, _ := json.Marshal(back)
	fmt.Println(err, string(reencoded) == string(encoded), back.At.Equal(event.At), back.Took)
	err = json.Unmarshal([]byte(`{"at":"nope"}`), &back)
	fmt.Println(err)
	err = json.Unmarshal([]byte(`{"took":"1s"}`), &back)
	fmt.Println(err)
	err = json.Unmarshal([]byte(`{"month":"March"}`), &back)
	fmt.Println(err)
	err = json.Unmarshal([]byte(`{"byName":{"x":"2020-01-01T00:00:00Z","y":5}}`), &back)
	fmt.Println(err, len(back.ByName))

	fmt.Println("== Duration and errors")
	for _, duration := range []time.Duration{0, 1, time.Second, -time.Hour, 1<<63 - 1, -1 << 63} {
		encoded, err := json.Marshal(duration)
		fmt.Printf("%v => %s %v\n", duration, encoded, err)
	}
	encoded, err = json.Marshal(map[string]any{"err": fmt.Errorf("boom"), "dur": time.Second, "month": time.Month(13), "loc": time.UTC})
	fmt.Printf("%s %v\n", encoded, err)
	encoded, err = json.Marshal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Weekday())
	fmt.Printf("%s %v\n", encoded, err)
}

// Output:
// == time.Time encode
// 2024-02-29T23:59:59Z | "2024-02-29T23:59:59Z" <nil>
// 2024-02-29T23:59:59.123456789Z | "2024-02-29T23:59:59.123456789Z" <nil>
// 2024-02-29T23:59:59.12Z | "2024-02-29T23:59:59.12Z" <nil>
// 2024-02-29T12:00:00-03:30 | "2024-02-29T12:00:00-03:30" <nil>
// 2024-02-29T12:00:00.000001+05:00 | "2024-02-29T12:00:00.000001+05:00" <nil>
// 0001-01-01T00:00:00Z | "0001-01-01T00:00:00Z" <nil>
// 9999-12-31T23:59:59.999999999Z | "9999-12-31T23:59:59.999999999Z" <nil>
// 0000-01-01T00:00:00Z | "0000-01-01T00:00:00Z" <nil>
// -0001-01-01T00:00:00Z |  json: error calling MarshalJSON for type time.Time: Time.MarshalJSON: year outside of range [0,9999]
// 10000-01-01T00:00:00Z |  json: error calling MarshalJSON for type time.Time: Time.MarshalJSON: year outside of range [0,9999]
// 2000-01-01T00:00:00+24:00 |  json: error calling MarshalJSON for type time.Time: Time.MarshalJSON: timezone hour outside of range [0,23]
// 2000-01-01T00:00:00+00:00 | "2000-01-01T00:00:00+00:00" <nil>
// 1970-01-01T00:00:00Z | "1970-01-01T00:00:00Z" <nil>
// 0001-01-01T00:00:00Z | "0001-01-01T00:00:00Z" <nil>
// == time.Time decode
// "2024-02-29T23:59:59Z"                   => 2024-02-29T23:59:59Z offset=0 err=<nil>
// "2024-02-29T23:59:59.5Z"                 => 2024-02-29T23:59:59.5Z offset=0 err=<nil>
// "2024-02-29T23:59:59.123456789012Z"      => 2024-02-29T23:59:59.123456789Z offset=0 err=<nil>
// "2024-02-29T23:59:59+05:30"              => 2024-02-29T23:59:59+05:30 offset=19800 err=<nil>
// "2024-02-29T23:59:59-00:00"              => 2024-02-29T23:59:59Z offset=0 err=<nil>
// "2024-02-29t23:59:59z"                   => 0001-01-01T00:00:00Z offset=0 err=parsing time "2024-02-29t23:59:59z" as "2006-01-02T15:04:05Z07:00": cannot parse "t23:59:59z" as "T"
// "2024-02-29 23:59:59Z"                   => 0001-01-01T00:00:00Z offset=0 err=parsing time "2024-02-29 23:59:59Z" as "2006-01-02T15:04:05Z07:00": cannot parse " 23:59:59Z" as "T"
// "2024-02-30T00:00:00Z"                   => 0001-01-01T00:00:00Z offset=0 err=parsing time "2024-02-30T00:00:00Z": day out of range
// "2023-02-29T00:00:00Z"                   => 0001-01-01T00:00:00Z offset=0 err=parsing time "2023-02-29T00:00:00Z": day out of range
// "2024-13-01T00:00:00Z"                   => 0001-01-01T00:00:00Z offset=0 err=parsing time "2024-13-01T00:00:00Z": month out of range
// "2024-01-01T24:00:00Z"                   => 0001-01-01T00:00:00Z offset=0 err=parsing time "2024-01-01T24:00:00Z": hour out of range
// "2024-01-01T23:60:00Z"                   => 0001-01-01T00:00:00Z offset=0 err=parsing time "2024-01-01T23:60:00Z": minute out of range
// "2024-01-01T23:59:60Z"                   => 0001-01-01T00:00:00Z offset=0 err=parsing time "2024-01-01T23:59:60Z": second out of range
// "2024-01-01T00:00:00"                    => 0001-01-01T00:00:00Z offset=0 err=parsing time "2024-01-01T00:00:00" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "Z07:00"
// "2024-01-01"                             => 0001-01-01T00:00:00Z offset=0 err=parsing time "2024-01-01" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "T"
// ""                                       => 0001-01-01T00:00:00Z offset=0 err=parsing time "" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "2006"
// "x"                                      => 0001-01-01T00:00:00Z offset=0 err=parsing time "x" as "2006-01-02T15:04:05Z07:00": cannot parse "x" as "2006"
// null                                     => 0001-01-01T00:00:00Z offset=0 err=<nil>
// 1                                        => 0001-01-01T00:00:00Z offset=0 err=Time.UnmarshalJSON: input is not a JSON string
// []                                       => 0001-01-01T00:00:00Z offset=0 err=Time.UnmarshalJSON: input is not a JSON string
// "2024-01-01T00:00:00+24:00"              => 2024-01-01T00:00:00+24:00 offset=86400 err=<nil>
// "2024-01-01T00:00:00+23:59"              => 2024-01-01T00:00:00+23:59 offset=86340 err=<nil>
// "0000-01-01T00:00:00Z"                   => 0000-01-01T00:00:00Z offset=0 err=<nil>
// "2024-01-01T00:00:00,5Z"                 => 2024-01-01T00:00:00.5Z offset=0 err=<nil>
// "2024-01-01T00:00:00.Z"                  => 0001-01-01T00:00:00Z offset=0 err=parsing time "2024-01-01T00:00:00.Z" as "2006-01-02T15:04:05Z07:00": cannot parse ".Z" as "Z07:00"
// "+2024-01-01T00:00:00Z"                  => 0001-01-01T00:00:00Z offset=0 err=parsing time "+2024-01-01T00:00:00Z" as "2006-01-02T15:04:05Z07:00": cannot parse "+2024-01-01T00:00:00Z" as "2006"
// "2024-01-01T00:00:00Z "                  => 0001-01-01T00:00:00Z offset=0 err=parsing time "2024-01-01T00:00:00Z ": extra text: " "
// " 2024-01-01T00:00:00Z"                  => 0001-01-01T00:00:00Z offset=0 err=parsing time " 2024-01-01T00:00:00Z" as "2006-01-02T15:04:05Z07:00": cannot parse " 2024-01-01T00:00:00Z" as "2006"
// == struct
// {"at":"2020-05-17T08:30:00.0000005-03:30","maybe":"2020-05-17T08:30:00.0000005-03:30","took":1500000000,"month":3,"day":5,"optional":["2020-05-17T08:30:00.0000005-03:30","0001-01-01T00:00:00Z"],"byName":{"a":"0001-01-01T00:00:00Z","b":"2020-05-17T08:30:00.0000005-03:30"}} <nil>
// <nil> true true 1.5s
// parsing time "nope" as "2006-01-02T15:04:05Z07:00": cannot parse "nope" as "2006"
// json: cannot unmarshal string into Go struct field Event.took of type time.Duration
// json: cannot unmarshal string into Go struct field Event.month of type time.Month
// Time.UnmarshalJSON: input is not a JSON string 3
// == Duration and errors
// 0s => 0 <nil>
// 1ns => 1 <nil>
// 1s => 1000000000 <nil>
// -1h0m0s => -3600000000000 <nil>
// 2562047h47m16.854775807s => 9223372036854775807 <nil>
// -2562047h47m16.854775808s => -9223372036854775808 <nil>
// {"dur":1000000000,"err":{},"loc":{},"month":13} <nil>
// 1 <nil>
