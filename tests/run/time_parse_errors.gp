// phase: 08

// time.Parse errors: *ParseError fields and Go's exact messages.
package main

import (
	"errors"
	"fmt"
	"time"
)

func main() {
	cases := [][2]string{
		{time.RFC3339, ""},
		{time.RFC3339, "2006-01-02"},
		{time.RFC3339, "2006-01-02T15:04:05"},
		{time.RFC3339, "2006-13-02T15:04:05Z"},
		{time.RFC3339, "2006-00-02T15:04:05Z"},
		{time.RFC3339, "2006-01-32T15:04:05Z"},
		{time.RFC3339, "2006-01-00T15:04:05Z"},
		{time.RFC3339, "2006-02-30T15:04:05Z"},
		{time.RFC3339, "2023-02-29T15:04:05Z"},
		{time.RFC3339, "2006-01-02T24:04:05Z"},
		{time.RFC3339, "2006-01-02T15:60:05Z"},
		{time.RFC3339, "2006-01-02T15:04:60Z"},
		{time.RFC3339, "2006-01-02T15:04:05+25:00"},
		{time.RFC3339, "2006-01-02T15:04:05+07:60"},
		{time.RFC3339, "2006-01-02 15:04:05Z"},
		{time.RFC3339, "2006-01-02T15:04:05Zjunk"},
		{time.RFC3339, "2006-01-02T15:04:05z"},
		{time.RFC3339, "garbage"},
		{time.RFC3339, "20-01-02T15:04:05Z"},
		{time.DateOnly, "2006-1-2"},
		{time.DateOnly, "2006-01-02 extra"},
		{time.DateOnly, "06-01-02"},
		{time.DateOnly, "abcd-01-02"},
		{time.DateOnly, "2006/01/02"},
		{time.TimeOnly, "25:00:00"},
		{time.TimeOnly, "12:61:00"},
		{time.TimeOnly, "12:00:61"},
		{time.TimeOnly, "12:00"},
		{time.Kitchen, "13:00PM"},
		{time.Kitchen, "3:04XM"},
		{time.Kitchen, "0:04PM"},
		{time.RFC1123, "Mon, 02 Jan 2006 15:04:05"},
		{time.RFC1123, "Xxx, 02 Jan 2006 15:04:05 MST"},
		{time.RFC1123, "Mon, 02 Foo 2006 15:04:05 MST"},
		{time.RFC1123, "Mon, 32 Jan 2006 15:04:05 MST"},
		{time.RFC1123, "Mon, 02 Jan 2006 15:04:05 mst"},
		{"2006-01-02", "2006-01-02T"},
		{"Monday", "Funday"},
		{"January", "Marchuary"},
		{"Jan", "Mar2"},
		{"2006-01-02 -0700", "2006-01-02 +07"},
		{"2006-01-02 -07:00", "2006-01-02 +0700"},
		{"2006-01-02 Z07:00", "2006-01-02 +7:00"},
		{"2006-01-02 -0700", "2006-01-02 +2400"},
		{"2006-01-02 MST", "2006-01-02 MS"},
		{"05.000", "05.12"},
		{"05", "5"},
		{"002", "400"},
		{"002", "000"},
		{"2006-002", "2023-366"},
		{"2006 Jan 2", "2006 Feb 30"},
		{"15", "1"},
		{"3", "13"},
		{"PM", "AM"},
		{"PM", "XX"},
		{"1", "13"},
		{"1", "0"},
		{"01", "00"},
		{"02", "00"},
		{"2", "32"},
		{"04", "60"},
		{"05", "60"},
		{"05", "-1"},
		{"2006", "20x6"},
		{"2006", "999"},
		{"06", "x1"},
		{"Mon Jan _2", "Mon Jan  x"},
		{"x", "y"},
		{"x", "x"},
		{"x", "xx"},
		{"", "x"},
		{"", ""},
	}
	for _, c := range cases {
		t, err := time.Parse(c[0], c[1])
		fmt.Printf("%q %q: ", c[0], c[1])
		if err == nil {
			fmt.Println("ok", t.UTC().Format(time.RFC3339Nano))
			continue
		}
		var pe *time.ParseError
		if errors.As(err, &pe) {
			fmt.Printf("%v | layout=%q value=%q elem=%q valueElem=%q msg=%q\n", err, pe.Layout, pe.Value, pe.LayoutElem, pe.ValueElem, pe.Message)
		} else {
			fmt.Printf("other error: %v\n", err)
		}
	}
}

// Output:
// "2006-01-02T15:04:05Z07:00" "": parsing time "" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "2006" | layout="2006-01-02T15:04:05Z07:00" value="" elem="2006" valueElem="" msg=""
// "2006-01-02T15:04:05Z07:00" "2006-01-02": parsing time "2006-01-02" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "T" | layout="2006-01-02T15:04:05Z07:00" value="2006-01-02" elem="T" valueElem="" msg=""
// "2006-01-02T15:04:05Z07:00" "2006-01-02T15:04:05": parsing time "2006-01-02T15:04:05" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "Z07:00" | layout="2006-01-02T15:04:05Z07:00" value="2006-01-02T15:04:05" elem="Z07:00" valueElem="" msg=""
// "2006-01-02T15:04:05Z07:00" "2006-13-02T15:04:05Z": parsing time "2006-13-02T15:04:05Z": month out of range | layout="2006-01-02T15:04:05Z07:00" value="2006-13-02T15:04:05Z" elem="01" valueElem="-02T15:04:05Z" msg=": month out of range"
// "2006-01-02T15:04:05Z07:00" "2006-00-02T15:04:05Z": parsing time "2006-00-02T15:04:05Z": month out of range | layout="2006-01-02T15:04:05Z07:00" value="2006-00-02T15:04:05Z" elem="01" valueElem="-02T15:04:05Z" msg=": month out of range"
// "2006-01-02T15:04:05Z07:00" "2006-01-32T15:04:05Z": parsing time "2006-01-32T15:04:05Z": day out of range | layout="2006-01-02T15:04:05Z07:00" value="2006-01-32T15:04:05Z" elem="" valueElem="" msg=": day out of range"
// "2006-01-02T15:04:05Z07:00" "2006-01-00T15:04:05Z": parsing time "2006-01-00T15:04:05Z": day out of range | layout="2006-01-02T15:04:05Z07:00" value="2006-01-00T15:04:05Z" elem="" valueElem="" msg=": day out of range"
// "2006-01-02T15:04:05Z07:00" "2006-02-30T15:04:05Z": parsing time "2006-02-30T15:04:05Z": day out of range | layout="2006-01-02T15:04:05Z07:00" value="2006-02-30T15:04:05Z" elem="" valueElem="" msg=": day out of range"
// "2006-01-02T15:04:05Z07:00" "2023-02-29T15:04:05Z": parsing time "2023-02-29T15:04:05Z": day out of range | layout="2006-01-02T15:04:05Z07:00" value="2023-02-29T15:04:05Z" elem="" valueElem="" msg=": day out of range"
// "2006-01-02T15:04:05Z07:00" "2006-01-02T24:04:05Z": parsing time "2006-01-02T24:04:05Z": hour out of range | layout="2006-01-02T15:04:05Z07:00" value="2006-01-02T24:04:05Z" elem="15" valueElem=":04:05Z" msg=": hour out of range"
// "2006-01-02T15:04:05Z07:00" "2006-01-02T15:60:05Z": parsing time "2006-01-02T15:60:05Z": minute out of range | layout="2006-01-02T15:04:05Z07:00" value="2006-01-02T15:60:05Z" elem="04" valueElem=":05Z" msg=": minute out of range"
// "2006-01-02T15:04:05Z07:00" "2006-01-02T15:04:60Z": parsing time "2006-01-02T15:04:60Z": second out of range | layout="2006-01-02T15:04:05Z07:00" value="2006-01-02T15:04:60Z" elem="05" valueElem="Z" msg=": second out of range"
// "2006-01-02T15:04:05Z07:00" "2006-01-02T15:04:05+25:00": parsing time "2006-01-02T15:04:05+25:00": time zone offset hour out of range | layout="2006-01-02T15:04:05Z07:00" value="2006-01-02T15:04:05+25:00" elem="Z07:00" valueElem="" msg=": time zone offset hour out of range"
// "2006-01-02T15:04:05Z07:00" "2006-01-02T15:04:05+07:60": ok 2006-01-02T07:04:05Z
// "2006-01-02T15:04:05Z07:00" "2006-01-02 15:04:05Z": parsing time "2006-01-02 15:04:05Z" as "2006-01-02T15:04:05Z07:00": cannot parse " 15:04:05Z" as "T" | layout="2006-01-02T15:04:05Z07:00" value="2006-01-02 15:04:05Z" elem="T" valueElem=" 15:04:05Z" msg=""
// "2006-01-02T15:04:05Z07:00" "2006-01-02T15:04:05Zjunk": parsing time "2006-01-02T15:04:05Zjunk": extra text: "junk" | layout="2006-01-02T15:04:05Z07:00" value="2006-01-02T15:04:05Zjunk" elem="" valueElem="junk" msg=": extra text: \"junk\""
// "2006-01-02T15:04:05Z07:00" "2006-01-02T15:04:05z": parsing time "2006-01-02T15:04:05z" as "2006-01-02T15:04:05Z07:00": cannot parse "z" as "Z07:00" | layout="2006-01-02T15:04:05Z07:00" value="2006-01-02T15:04:05z" elem="Z07:00" valueElem="z" msg=""
// "2006-01-02T15:04:05Z07:00" "garbage": parsing time "garbage" as "2006-01-02T15:04:05Z07:00": cannot parse "garbage" as "2006" | layout="2006-01-02T15:04:05Z07:00" value="garbage" elem="2006" valueElem="garbage" msg=""
// "2006-01-02T15:04:05Z07:00" "20-01-02T15:04:05Z": parsing time "20-01-02T15:04:05Z" as "2006-01-02T15:04:05Z07:00": cannot parse "20-01-02T15:04:05Z" as "2006" | layout="2006-01-02T15:04:05Z07:00" value="20-01-02T15:04:05Z" elem="2006" valueElem="20-01-02T15:04:05Z" msg=""
// "2006-01-02" "2006-1-2": parsing time "2006-1-2" as "2006-01-02": cannot parse "1-2" as "01" | layout="2006-01-02" value="2006-1-2" elem="01" valueElem="1-2" msg=""
// "2006-01-02" "2006-01-02 extra": parsing time "2006-01-02 extra": extra text: " extra" | layout="2006-01-02" value="2006-01-02 extra" elem="" valueElem=" extra" msg=": extra text: \" extra\""
// "2006-01-02" "06-01-02": parsing time "06-01-02" as "2006-01-02": cannot parse "06-01-02" as "2006" | layout="2006-01-02" value="06-01-02" elem="2006" valueElem="06-01-02" msg=""
// "2006-01-02" "abcd-01-02": parsing time "abcd-01-02" as "2006-01-02": cannot parse "abcd-01-02" as "2006" | layout="2006-01-02" value="abcd-01-02" elem="2006" valueElem="abcd-01-02" msg=""
// "2006-01-02" "2006/01/02": parsing time "2006/01/02" as "2006-01-02": cannot parse "/01/02" as "-" | layout="2006-01-02" value="2006/01/02" elem="-" valueElem="/01/02" msg=""
// "15:04:05" "25:00:00": parsing time "25:00:00": hour out of range | layout="15:04:05" value="25:00:00" elem="15" valueElem=":00:00" msg=": hour out of range"
// "15:04:05" "12:61:00": parsing time "12:61:00": minute out of range | layout="15:04:05" value="12:61:00" elem="04" valueElem=":00" msg=": minute out of range"
// "15:04:05" "12:00:61": parsing time "12:00:61": second out of range | layout="15:04:05" value="12:00:61" elem="05" valueElem="" msg=": second out of range"
// "15:04:05" "12:00": parsing time "12:00" as "15:04:05": cannot parse "" as ":" | layout="15:04:05" value="12:00" elem=":" valueElem="" msg=""
// "3:04PM" "13:00PM": parsing time "13:00PM": hour out of range | layout="3:04PM" value="13:00PM" elem="3" valueElem=":00PM" msg=": hour out of range"
// "3:04PM" "3:04XM": parsing time "3:04XM" as "3:04PM": cannot parse "XM" as "PM" | layout="3:04PM" value="3:04XM" elem="PM" valueElem="XM" msg=""
// "3:04PM" "0:04PM": ok 0000-01-01T12:04:00Z
// "Mon, 02 Jan 2006 15:04:05 MST" "Mon, 02 Jan 2006 15:04:05": parsing time "Mon, 02 Jan 2006 15:04:05" as "Mon, 02 Jan 2006 15:04:05 MST": cannot parse "" as "MST" | layout="Mon, 02 Jan 2006 15:04:05 MST" value="Mon, 02 Jan 2006 15:04:05" elem="MST" valueElem="" msg=""
// "Mon, 02 Jan 2006 15:04:05 MST" "Xxx, 02 Jan 2006 15:04:05 MST": parsing time "Xxx, 02 Jan 2006 15:04:05 MST" as "Mon, 02 Jan 2006 15:04:05 MST": cannot parse "Xxx, 02 Jan 2006 15:04:05 MST" as "Mon" | layout="Mon, 02 Jan 2006 15:04:05 MST" value="Xxx, 02 Jan 2006 15:04:05 MST" elem="Mon" valueElem="Xxx, 02 Jan 2006 15:04:05 MST" msg=""
// "Mon, 02 Jan 2006 15:04:05 MST" "Mon, 02 Foo 2006 15:04:05 MST": parsing time "Mon, 02 Foo 2006 15:04:05 MST" as "Mon, 02 Jan 2006 15:04:05 MST": cannot parse "Foo 2006 15:04:05 MST" as "Jan" | layout="Mon, 02 Jan 2006 15:04:05 MST" value="Mon, 02 Foo 2006 15:04:05 MST" elem="Jan" valueElem="Foo 2006 15:04:05 MST" msg=""
// "Mon, 02 Jan 2006 15:04:05 MST" "Mon, 32 Jan 2006 15:04:05 MST": parsing time "Mon, 32 Jan 2006 15:04:05 MST": day out of range | layout="Mon, 02 Jan 2006 15:04:05 MST" value="Mon, 32 Jan 2006 15:04:05 MST" elem="" valueElem="" msg=": day out of range"
// "Mon, 02 Jan 2006 15:04:05 MST" "Mon, 02 Jan 2006 15:04:05 mst": parsing time "Mon, 02 Jan 2006 15:04:05 mst" as "Mon, 02 Jan 2006 15:04:05 MST": cannot parse "mst" as "MST" | layout="Mon, 02 Jan 2006 15:04:05 MST" value="Mon, 02 Jan 2006 15:04:05 mst" elem="MST" valueElem="mst" msg=""
// "2006-01-02" "2006-01-02T": parsing time "2006-01-02T": extra text: "T" | layout="2006-01-02" value="2006-01-02T" elem="" valueElem="T" msg=": extra text: \"T\""
// "Monday" "Funday": parsing time "Funday" as "Monday": cannot parse "Funday" as "Monday" | layout="Monday" value="Funday" elem="Monday" valueElem="Funday" msg=""
// "January" "Marchuary": parsing time "Marchuary": extra text: "uary" | layout="January" value="Marchuary" elem="" valueElem="uary" msg=": extra text: \"uary\""
// "Jan" "Mar2": parsing time "Mar2": extra text: "2" | layout="Jan" value="Mar2" elem="" valueElem="2" msg=": extra text: \"2\""
// "2006-01-02 -0700" "2006-01-02 +07": parsing time "2006-01-02 +07" as "2006-01-02 -0700": cannot parse "+07" as "-0700" | layout="2006-01-02 -0700" value="2006-01-02 +07" elem="-0700" valueElem="+07" msg=""
// "2006-01-02 -07:00" "2006-01-02 +0700": parsing time "2006-01-02 +0700" as "2006-01-02 -07:00": cannot parse "+0700" as "-07:00" | layout="2006-01-02 -07:00" value="2006-01-02 +0700" elem="-07:00" valueElem="+0700" msg=""
// "2006-01-02 Z07:00" "2006-01-02 +7:00": parsing time "2006-01-02 +7:00" as "2006-01-02 Z07:00": cannot parse "+7:00" as "Z07:00" | layout="2006-01-02 Z07:00" value="2006-01-02 +7:00" elem="Z07:00" valueElem="+7:00" msg=""
// "2006-01-02 -0700" "2006-01-02 +2400": ok 2006-01-01T00:00:00Z
// "2006-01-02 MST" "2006-01-02 MS": parsing time "2006-01-02 MS" as "2006-01-02 MST": cannot parse "MS" as "MST" | layout="2006-01-02 MST" value="2006-01-02 MS" elem="MST" valueElem="MS" msg=""
// "05.000" "05.12": parsing time "05.12" as "05.000": cannot parse ".12" as ".000" | layout="05.000" value="05.12" elem=".000" valueElem=".12" msg=""
// "05" "5": parsing time "5" as "05": cannot parse "5" as "05" | layout="05" value="5" elem="05" valueElem="5" msg=""
// "002" "400": parsing time "400": day-of-year out of range | layout="002" value="400" elem="" valueElem="" msg=": day-of-year out of range"
// "002" "000": parsing time "000": day-of-year out of range | layout="002" value="000" elem="" valueElem="" msg=": day-of-year out of range"
// "2006-002" "2023-366": parsing time "2023-366": day-of-year out of range | layout="2006-002" value="2023-366" elem="" valueElem="" msg=": day-of-year out of range"
// "2006 Jan 2" "2006 Feb 30": parsing time "2006 Feb 30": day out of range | layout="2006 Jan 2" value="2006 Feb 30" elem="" valueElem="" msg=": day out of range"
// "15" "1": ok 0000-01-01T01:00:00Z
// "3" "13": parsing time "13": hour out of range | layout="3" value="13" elem="3" valueElem="" msg=": hour out of range"
// "PM" "AM": ok 0000-01-01T00:00:00Z
// "PM" "XX": parsing time "XX" as "PM": cannot parse "XX" as "PM" | layout="PM" value="XX" elem="PM" valueElem="XX" msg=""
// "1" "13": parsing time "13": month out of range | layout="1" value="13" elem="1" valueElem="" msg=": month out of range"
// "1" "0": parsing time "0": month out of range | layout="1" value="0" elem="1" valueElem="" msg=": month out of range"
// "01" "00": parsing time "00": month out of range | layout="01" value="00" elem="01" valueElem="" msg=": month out of range"
// "02" "00": parsing time "00": day out of range | layout="02" value="00" elem="" valueElem="" msg=": day out of range"
// "2" "32": parsing time "32": day out of range | layout="2" value="32" elem="" valueElem="" msg=": day out of range"
// "04" "60": parsing time "60": minute out of range | layout="04" value="60" elem="04" valueElem="" msg=": minute out of range"
// "05" "60": parsing time "60": second out of range | layout="05" value="60" elem="05" valueElem="" msg=": second out of range"
// "05" "-1": parsing time "-1" as "05": cannot parse "-1" as "05" | layout="05" value="-1" elem="05" valueElem="-1" msg=""
// "2006" "20x6": parsing time "20x6" as "2006": cannot parse "20x6" as "2006" | layout="2006" value="20x6" elem="2006" valueElem="20x6" msg=""
// "2006" "999": parsing time "999" as "2006": cannot parse "999" as "2006" | layout="2006" value="999" elem="2006" valueElem="999" msg=""
// "06" "x1": parsing time "x1" as "06": cannot parse "x1" as "06" | layout="06" value="x1" elem="06" valueElem="x1" msg=""
// "Mon Jan _2" "Mon Jan  x": parsing time "Mon Jan  x" as "Mon Jan _2": cannot parse "x" as "_2" | layout="Mon Jan _2" value="Mon Jan  x" elem="_2" valueElem="x" msg=""
// "x" "y": parsing time "y" as "x": cannot parse "y" as "x" | layout="x" value="y" elem="x" valueElem="y" msg=""
// "x" "x": ok 0000-01-01T00:00:00Z
// "x" "xx": parsing time "xx": extra text: "x" | layout="x" value="xx" elem="" valueElem="x" msg=": extra text: \"x\""
// "" "x": parsing time "x": extra text: "x" | layout="" value="x" elem="" valueElem="x" msg=": extra text: \"x\""
// "" "": ok 0000-01-01T00:00:00Z
