// phase: 08

// The predefined layouts of package time applied to fixed times in UTC and in fixed zones.
package main

import (
	"fmt"
	"time"
)

func main() {
	zones := []*time.Location{time.UTC, time.FixedZone("EST", -5*3600), time.FixedZone("IST", 5*3600+1800), time.FixedZone("", 3600), time.FixedZone("NEG", -(9*3600 + 45*60 + 30))}
	times := []time.Time{
		time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC),
		time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
		time.Date(1999, 12, 31, 23, 59, 59, 999999999, time.UTC),
		time.Date(2001, 9, 9, 1, 46, 40, 123456000, time.UTC),
		time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	layouts := []struct {
		name, layout string
	}{
		{"Layout", time.Layout}, {"ANSIC", time.ANSIC}, {"UnixDate", time.UnixDate}, {"RubyDate", time.RubyDate},
		{"RFC822", time.RFC822}, {"RFC822Z", time.RFC822Z}, {"RFC850", time.RFC850}, {"RFC1123", time.RFC1123},
		{"RFC1123Z", time.RFC1123Z}, {"RFC3339", time.RFC3339}, {"RFC3339Nano", time.RFC3339Nano}, {"Kitchen", time.Kitchen},
		{"Stamp", time.Stamp}, {"StampMilli", time.StampMilli}, {"StampMicro", time.StampMicro}, {"StampNano", time.StampNano},
		{"DateTime", time.DateTime}, {"DateOnly", time.DateOnly}, {"TimeOnly", time.TimeOnly},
	}
	for _, l := range layouts {
		fmt.Printf("%s = %q\n", l.name, l.layout)
	}
	for _, t := range times {
		for zi, z := range zones {
			tt := t.In(z)
			fmt.Printf("--- %d %s\n", zi, tt.Format("2006-01-02T15:04:05.999999999Z07:00"))
			for _, l := range layouts {
				fmt.Printf("%-12s %s\n", l.name, tt.Format(l.layout))
			}
		}
	}
}

// Output:
// Layout = "01/02 03:04:05PM '06 -0700"
// ANSIC = "Mon Jan _2 15:04:05 2006"
// UnixDate = "Mon Jan _2 15:04:05 MST 2006"
// RubyDate = "Mon Jan 02 15:04:05 -0700 2006"
// RFC822 = "02 Jan 06 15:04 MST"
// RFC822Z = "02 Jan 06 15:04 -0700"
// RFC850 = "Monday, 02-Jan-06 15:04:05 MST"
// RFC1123 = "Mon, 02 Jan 2006 15:04:05 MST"
// RFC1123Z = "Mon, 02 Jan 2006 15:04:05 -0700"
// RFC3339 = "2006-01-02T15:04:05Z07:00"
// RFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"
// Kitchen = "3:04PM"
// Stamp = "Jan _2 15:04:05"
// StampMilli = "Jan _2 15:04:05.000"
// StampMicro = "Jan _2 15:04:05.000000"
// StampNano = "Jan _2 15:04:05.000000000"
// DateTime = "2006-01-02 15:04:05"
// DateOnly = "2006-01-02"
// TimeOnly = "15:04:05"
// --- 0 2006-01-02T15:04:05Z
// Layout       01/02 03:04:05PM '06 +0000
// ANSIC        Mon Jan  2 15:04:05 2006
// UnixDate     Mon Jan  2 15:04:05 UTC 2006
// RubyDate     Mon Jan 02 15:04:05 +0000 2006
// RFC822       02 Jan 06 15:04 UTC
// RFC822Z      02 Jan 06 15:04 +0000
// RFC850       Monday, 02-Jan-06 15:04:05 UTC
// RFC1123      Mon, 02 Jan 2006 15:04:05 UTC
// RFC1123Z     Mon, 02 Jan 2006 15:04:05 +0000
// RFC3339      2006-01-02T15:04:05Z
// RFC3339Nano  2006-01-02T15:04:05Z
// Kitchen      3:04PM
// Stamp        Jan  2 15:04:05
// StampMilli   Jan  2 15:04:05.000
// StampMicro   Jan  2 15:04:05.000000
// StampNano    Jan  2 15:04:05.000000000
// DateTime     2006-01-02 15:04:05
// DateOnly     2006-01-02
// TimeOnly     15:04:05
// --- 1 2006-01-02T10:04:05-05:00
// Layout       01/02 10:04:05AM '06 -0500
// ANSIC        Mon Jan  2 10:04:05 2006
// UnixDate     Mon Jan  2 10:04:05 EST 2006
// RubyDate     Mon Jan 02 10:04:05 -0500 2006
// RFC822       02 Jan 06 10:04 EST
// RFC822Z      02 Jan 06 10:04 -0500
// RFC850       Monday, 02-Jan-06 10:04:05 EST
// RFC1123      Mon, 02 Jan 2006 10:04:05 EST
// RFC1123Z     Mon, 02 Jan 2006 10:04:05 -0500
// RFC3339      2006-01-02T10:04:05-05:00
// RFC3339Nano  2006-01-02T10:04:05-05:00
// Kitchen      10:04AM
// Stamp        Jan  2 10:04:05
// StampMilli   Jan  2 10:04:05.000
// StampMicro   Jan  2 10:04:05.000000
// StampNano    Jan  2 10:04:05.000000000
// DateTime     2006-01-02 10:04:05
// DateOnly     2006-01-02
// TimeOnly     10:04:05
// --- 2 2006-01-02T20:34:05+05:30
// Layout       01/02 08:34:05PM '06 +0530
// ANSIC        Mon Jan  2 20:34:05 2006
// UnixDate     Mon Jan  2 20:34:05 IST 2006
// RubyDate     Mon Jan 02 20:34:05 +0530 2006
// RFC822       02 Jan 06 20:34 IST
// RFC822Z      02 Jan 06 20:34 +0530
// RFC850       Monday, 02-Jan-06 20:34:05 IST
// RFC1123      Mon, 02 Jan 2006 20:34:05 IST
// RFC1123Z     Mon, 02 Jan 2006 20:34:05 +0530
// RFC3339      2006-01-02T20:34:05+05:30
// RFC3339Nano  2006-01-02T20:34:05+05:30
// Kitchen      8:34PM
// Stamp        Jan  2 20:34:05
// StampMilli   Jan  2 20:34:05.000
// StampMicro   Jan  2 20:34:05.000000
// StampNano    Jan  2 20:34:05.000000000
// DateTime     2006-01-02 20:34:05
// DateOnly     2006-01-02
// TimeOnly     20:34:05
// --- 3 2006-01-02T16:04:05+01:00
// Layout       01/02 04:04:05PM '06 +0100
// ANSIC        Mon Jan  2 16:04:05 2006
// UnixDate     Mon Jan  2 16:04:05 +0100 2006
// RubyDate     Mon Jan 02 16:04:05 +0100 2006
// RFC822       02 Jan 06 16:04 +0100
// RFC822Z      02 Jan 06 16:04 +0100
// RFC850       Monday, 02-Jan-06 16:04:05 +0100
// RFC1123      Mon, 02 Jan 2006 16:04:05 +0100
// RFC1123Z     Mon, 02 Jan 2006 16:04:05 +0100
// RFC3339      2006-01-02T16:04:05+01:00
// RFC3339Nano  2006-01-02T16:04:05+01:00
// Kitchen      4:04PM
// Stamp        Jan  2 16:04:05
// StampMilli   Jan  2 16:04:05.000
// StampMicro   Jan  2 16:04:05.000000
// StampNano    Jan  2 16:04:05.000000000
// DateTime     2006-01-02 16:04:05
// DateOnly     2006-01-02
// TimeOnly     16:04:05
// --- 4 2006-01-02T05:18:35-09:45
// Layout       01/02 05:18:35AM '06 -0945
// ANSIC        Mon Jan  2 05:18:35 2006
// UnixDate     Mon Jan  2 05:18:35 NEG 2006
// RubyDate     Mon Jan 02 05:18:35 -0945 2006
// RFC822       02 Jan 06 05:18 NEG
// RFC822Z      02 Jan 06 05:18 -0945
// RFC850       Monday, 02-Jan-06 05:18:35 NEG
// RFC1123      Mon, 02 Jan 2006 05:18:35 NEG
// RFC1123Z     Mon, 02 Jan 2006 05:18:35 -0945
// RFC3339      2006-01-02T05:18:35-09:45
// RFC3339Nano  2006-01-02T05:18:35-09:45
// Kitchen      5:18AM
// Stamp        Jan  2 05:18:35
// StampMilli   Jan  2 05:18:35.000
// StampMicro   Jan  2 05:18:35.000000
// StampNano    Jan  2 05:18:35.000000000
// DateTime     2006-01-02 05:18:35
// DateOnly     2006-01-02
// TimeOnly     05:18:35
// --- 0 2024-02-29T00:00:00Z
// Layout       02/29 12:00:00AM '24 +0000
// ANSIC        Thu Feb 29 00:00:00 2024
// UnixDate     Thu Feb 29 00:00:00 UTC 2024
// RubyDate     Thu Feb 29 00:00:00 +0000 2024
// RFC822       29 Feb 24 00:00 UTC
// RFC822Z      29 Feb 24 00:00 +0000
// RFC850       Thursday, 29-Feb-24 00:00:00 UTC
// RFC1123      Thu, 29 Feb 2024 00:00:00 UTC
// RFC1123Z     Thu, 29 Feb 2024 00:00:00 +0000
// RFC3339      2024-02-29T00:00:00Z
// RFC3339Nano  2024-02-29T00:00:00Z
// Kitchen      12:00AM
// Stamp        Feb 29 00:00:00
// StampMilli   Feb 29 00:00:00.000
// StampMicro   Feb 29 00:00:00.000000
// StampNano    Feb 29 00:00:00.000000000
// DateTime     2024-02-29 00:00:00
// DateOnly     2024-02-29
// TimeOnly     00:00:00
// --- 1 2024-02-28T19:00:00-05:00
// Layout       02/28 07:00:00PM '24 -0500
// ANSIC        Wed Feb 28 19:00:00 2024
// UnixDate     Wed Feb 28 19:00:00 EST 2024
// RubyDate     Wed Feb 28 19:00:00 -0500 2024
// RFC822       28 Feb 24 19:00 EST
// RFC822Z      28 Feb 24 19:00 -0500
// RFC850       Wednesday, 28-Feb-24 19:00:00 EST
// RFC1123      Wed, 28 Feb 2024 19:00:00 EST
// RFC1123Z     Wed, 28 Feb 2024 19:00:00 -0500
// RFC3339      2024-02-28T19:00:00-05:00
// RFC3339Nano  2024-02-28T19:00:00-05:00
// Kitchen      7:00PM
// Stamp        Feb 28 19:00:00
// StampMilli   Feb 28 19:00:00.000
// StampMicro   Feb 28 19:00:00.000000
// StampNano    Feb 28 19:00:00.000000000
// DateTime     2024-02-28 19:00:00
// DateOnly     2024-02-28
// TimeOnly     19:00:00
// --- 2 2024-02-29T05:30:00+05:30
// Layout       02/29 05:30:00AM '24 +0530
// ANSIC        Thu Feb 29 05:30:00 2024
// UnixDate     Thu Feb 29 05:30:00 IST 2024
// RubyDate     Thu Feb 29 05:30:00 +0530 2024
// RFC822       29 Feb 24 05:30 IST
// RFC822Z      29 Feb 24 05:30 +0530
// RFC850       Thursday, 29-Feb-24 05:30:00 IST
// RFC1123      Thu, 29 Feb 2024 05:30:00 IST
// RFC1123Z     Thu, 29 Feb 2024 05:30:00 +0530
// RFC3339      2024-02-29T05:30:00+05:30
// RFC3339Nano  2024-02-29T05:30:00+05:30
// Kitchen      5:30AM
// Stamp        Feb 29 05:30:00
// StampMilli   Feb 29 05:30:00.000
// StampMicro   Feb 29 05:30:00.000000
// StampNano    Feb 29 05:30:00.000000000
// DateTime     2024-02-29 05:30:00
// DateOnly     2024-02-29
// TimeOnly     05:30:00
// --- 3 2024-02-29T01:00:00+01:00
// Layout       02/29 01:00:00AM '24 +0100
// ANSIC        Thu Feb 29 01:00:00 2024
// UnixDate     Thu Feb 29 01:00:00 +0100 2024
// RubyDate     Thu Feb 29 01:00:00 +0100 2024
// RFC822       29 Feb 24 01:00 +0100
// RFC822Z      29 Feb 24 01:00 +0100
// RFC850       Thursday, 29-Feb-24 01:00:00 +0100
// RFC1123      Thu, 29 Feb 2024 01:00:00 +0100
// RFC1123Z     Thu, 29 Feb 2024 01:00:00 +0100
// RFC3339      2024-02-29T01:00:00+01:00
// RFC3339Nano  2024-02-29T01:00:00+01:00
// Kitchen      1:00AM
// Stamp        Feb 29 01:00:00
// StampMilli   Feb 29 01:00:00.000
// StampMicro   Feb 29 01:00:00.000000
// StampNano    Feb 29 01:00:00.000000000
// DateTime     2024-02-29 01:00:00
// DateOnly     2024-02-29
// TimeOnly     01:00:00
// --- 4 2024-02-28T14:14:30-09:45
// Layout       02/28 02:14:30PM '24 -0945
// ANSIC        Wed Feb 28 14:14:30 2024
// UnixDate     Wed Feb 28 14:14:30 NEG 2024
// RubyDate     Wed Feb 28 14:14:30 -0945 2024
// RFC822       28 Feb 24 14:14 NEG
// RFC822Z      28 Feb 24 14:14 -0945
// RFC850       Wednesday, 28-Feb-24 14:14:30 NEG
// RFC1123      Wed, 28 Feb 2024 14:14:30 NEG
// RFC1123Z     Wed, 28 Feb 2024 14:14:30 -0945
// RFC3339      2024-02-28T14:14:30-09:45
// RFC3339Nano  2024-02-28T14:14:30-09:45
// Kitchen      2:14PM
// Stamp        Feb 28 14:14:30
// StampMilli   Feb 28 14:14:30.000
// StampMicro   Feb 28 14:14:30.000000
// StampNano    Feb 28 14:14:30.000000000
// DateTime     2024-02-28 14:14:30
// DateOnly     2024-02-28
// TimeOnly     14:14:30
// --- 0 1999-12-31T23:59:59.999999999Z
// Layout       12/31 11:59:59PM '99 +0000
// ANSIC        Fri Dec 31 23:59:59 1999
// UnixDate     Fri Dec 31 23:59:59 UTC 1999
// RubyDate     Fri Dec 31 23:59:59 +0000 1999
// RFC822       31 Dec 99 23:59 UTC
// RFC822Z      31 Dec 99 23:59 +0000
// RFC850       Friday, 31-Dec-99 23:59:59 UTC
// RFC1123      Fri, 31 Dec 1999 23:59:59 UTC
// RFC1123Z     Fri, 31 Dec 1999 23:59:59 +0000
// RFC3339      1999-12-31T23:59:59Z
// RFC3339Nano  1999-12-31T23:59:59.999999999Z
// Kitchen      11:59PM
// Stamp        Dec 31 23:59:59
// StampMilli   Dec 31 23:59:59.999
// StampMicro   Dec 31 23:59:59.999999
// StampNano    Dec 31 23:59:59.999999999
// DateTime     1999-12-31 23:59:59
// DateOnly     1999-12-31
// TimeOnly     23:59:59
// --- 1 1999-12-31T18:59:59.999999999-05:00
// Layout       12/31 06:59:59PM '99 -0500
// ANSIC        Fri Dec 31 18:59:59 1999
// UnixDate     Fri Dec 31 18:59:59 EST 1999
// RubyDate     Fri Dec 31 18:59:59 -0500 1999
// RFC822       31 Dec 99 18:59 EST
// RFC822Z      31 Dec 99 18:59 -0500
// RFC850       Friday, 31-Dec-99 18:59:59 EST
// RFC1123      Fri, 31 Dec 1999 18:59:59 EST
// RFC1123Z     Fri, 31 Dec 1999 18:59:59 -0500
// RFC3339      1999-12-31T18:59:59-05:00
// RFC3339Nano  1999-12-31T18:59:59.999999999-05:00
// Kitchen      6:59PM
// Stamp        Dec 31 18:59:59
// StampMilli   Dec 31 18:59:59.999
// StampMicro   Dec 31 18:59:59.999999
// StampNano    Dec 31 18:59:59.999999999
// DateTime     1999-12-31 18:59:59
// DateOnly     1999-12-31
// TimeOnly     18:59:59
// --- 2 2000-01-01T05:29:59.999999999+05:30
// Layout       01/01 05:29:59AM '00 +0530
// ANSIC        Sat Jan  1 05:29:59 2000
// UnixDate     Sat Jan  1 05:29:59 IST 2000
// RubyDate     Sat Jan 01 05:29:59 +0530 2000
// RFC822       01 Jan 00 05:29 IST
// RFC822Z      01 Jan 00 05:29 +0530
// RFC850       Saturday, 01-Jan-00 05:29:59 IST
// RFC1123      Sat, 01 Jan 2000 05:29:59 IST
// RFC1123Z     Sat, 01 Jan 2000 05:29:59 +0530
// RFC3339      2000-01-01T05:29:59+05:30
// RFC3339Nano  2000-01-01T05:29:59.999999999+05:30
// Kitchen      5:29AM
// Stamp        Jan  1 05:29:59
// StampMilli   Jan  1 05:29:59.999
// StampMicro   Jan  1 05:29:59.999999
// StampNano    Jan  1 05:29:59.999999999
// DateTime     2000-01-01 05:29:59
// DateOnly     2000-01-01
// TimeOnly     05:29:59
// --- 3 2000-01-01T00:59:59.999999999+01:00
// Layout       01/01 12:59:59AM '00 +0100
// ANSIC        Sat Jan  1 00:59:59 2000
// UnixDate     Sat Jan  1 00:59:59 +0100 2000
// RubyDate     Sat Jan 01 00:59:59 +0100 2000
// RFC822       01 Jan 00 00:59 +0100
// RFC822Z      01 Jan 00 00:59 +0100
// RFC850       Saturday, 01-Jan-00 00:59:59 +0100
// RFC1123      Sat, 01 Jan 2000 00:59:59 +0100
// RFC1123Z     Sat, 01 Jan 2000 00:59:59 +0100
// RFC3339      2000-01-01T00:59:59+01:00
// RFC3339Nano  2000-01-01T00:59:59.999999999+01:00
// Kitchen      12:59AM
// Stamp        Jan  1 00:59:59
// StampMilli   Jan  1 00:59:59.999
// StampMicro   Jan  1 00:59:59.999999
// StampNano    Jan  1 00:59:59.999999999
// DateTime     2000-01-01 00:59:59
// DateOnly     2000-01-01
// TimeOnly     00:59:59
// --- 4 1999-12-31T14:14:29.999999999-09:45
// Layout       12/31 02:14:29PM '99 -0945
// ANSIC        Fri Dec 31 14:14:29 1999
// UnixDate     Fri Dec 31 14:14:29 NEG 1999
// RubyDate     Fri Dec 31 14:14:29 -0945 1999
// RFC822       31 Dec 99 14:14 NEG
// RFC822Z      31 Dec 99 14:14 -0945
// RFC850       Friday, 31-Dec-99 14:14:29 NEG
// RFC1123      Fri, 31 Dec 1999 14:14:29 NEG
// RFC1123Z     Fri, 31 Dec 1999 14:14:29 -0945
// RFC3339      1999-12-31T14:14:29-09:45
// RFC3339Nano  1999-12-31T14:14:29.999999999-09:45
// Kitchen      2:14PM
// Stamp        Dec 31 14:14:29
// StampMilli   Dec 31 14:14:29.999
// StampMicro   Dec 31 14:14:29.999999
// StampNano    Dec 31 14:14:29.999999999
// DateTime     1999-12-31 14:14:29
// DateOnly     1999-12-31
// TimeOnly     14:14:29
// --- 0 2001-09-09T01:46:40.123456Z
// Layout       09/09 01:46:40AM '01 +0000
// ANSIC        Sun Sep  9 01:46:40 2001
// UnixDate     Sun Sep  9 01:46:40 UTC 2001
// RubyDate     Sun Sep 09 01:46:40 +0000 2001
// RFC822       09 Sep 01 01:46 UTC
// RFC822Z      09 Sep 01 01:46 +0000
// RFC850       Sunday, 09-Sep-01 01:46:40 UTC
// RFC1123      Sun, 09 Sep 2001 01:46:40 UTC
// RFC1123Z     Sun, 09 Sep 2001 01:46:40 +0000
// RFC3339      2001-09-09T01:46:40Z
// RFC3339Nano  2001-09-09T01:46:40.123456Z
// Kitchen      1:46AM
// Stamp        Sep  9 01:46:40
// StampMilli   Sep  9 01:46:40.123
// StampMicro   Sep  9 01:46:40.123456
// StampNano    Sep  9 01:46:40.123456000
// DateTime     2001-09-09 01:46:40
// DateOnly     2001-09-09
// TimeOnly     01:46:40
// --- 1 2001-09-08T20:46:40.123456-05:00
// Layout       09/08 08:46:40PM '01 -0500
// ANSIC        Sat Sep  8 20:46:40 2001
// UnixDate     Sat Sep  8 20:46:40 EST 2001
// RubyDate     Sat Sep 08 20:46:40 -0500 2001
// RFC822       08 Sep 01 20:46 EST
// RFC822Z      08 Sep 01 20:46 -0500
// RFC850       Saturday, 08-Sep-01 20:46:40 EST
// RFC1123      Sat, 08 Sep 2001 20:46:40 EST
// RFC1123Z     Sat, 08 Sep 2001 20:46:40 -0500
// RFC3339      2001-09-08T20:46:40-05:00
// RFC3339Nano  2001-09-08T20:46:40.123456-05:00
// Kitchen      8:46PM
// Stamp        Sep  8 20:46:40
// StampMilli   Sep  8 20:46:40.123
// StampMicro   Sep  8 20:46:40.123456
// StampNano    Sep  8 20:46:40.123456000
// DateTime     2001-09-08 20:46:40
// DateOnly     2001-09-08
// TimeOnly     20:46:40
// --- 2 2001-09-09T07:16:40.123456+05:30
// Layout       09/09 07:16:40AM '01 +0530
// ANSIC        Sun Sep  9 07:16:40 2001
// UnixDate     Sun Sep  9 07:16:40 IST 2001
// RubyDate     Sun Sep 09 07:16:40 +0530 2001
// RFC822       09 Sep 01 07:16 IST
// RFC822Z      09 Sep 01 07:16 +0530
// RFC850       Sunday, 09-Sep-01 07:16:40 IST
// RFC1123      Sun, 09 Sep 2001 07:16:40 IST
// RFC1123Z     Sun, 09 Sep 2001 07:16:40 +0530
// RFC3339      2001-09-09T07:16:40+05:30
// RFC3339Nano  2001-09-09T07:16:40.123456+05:30
// Kitchen      7:16AM
// Stamp        Sep  9 07:16:40
// StampMilli   Sep  9 07:16:40.123
// StampMicro   Sep  9 07:16:40.123456
// StampNano    Sep  9 07:16:40.123456000
// DateTime     2001-09-09 07:16:40
// DateOnly     2001-09-09
// TimeOnly     07:16:40
// --- 3 2001-09-09T02:46:40.123456+01:00
// Layout       09/09 02:46:40AM '01 +0100
// ANSIC        Sun Sep  9 02:46:40 2001
// UnixDate     Sun Sep  9 02:46:40 +0100 2001
// RubyDate     Sun Sep 09 02:46:40 +0100 2001
// RFC822       09 Sep 01 02:46 +0100
// RFC822Z      09 Sep 01 02:46 +0100
// RFC850       Sunday, 09-Sep-01 02:46:40 +0100
// RFC1123      Sun, 09 Sep 2001 02:46:40 +0100
// RFC1123Z     Sun, 09 Sep 2001 02:46:40 +0100
// RFC3339      2001-09-09T02:46:40+01:00
// RFC3339Nano  2001-09-09T02:46:40.123456+01:00
// Kitchen      2:46AM
// Stamp        Sep  9 02:46:40
// StampMilli   Sep  9 02:46:40.123
// StampMicro   Sep  9 02:46:40.123456
// StampNano    Sep  9 02:46:40.123456000
// DateTime     2001-09-09 02:46:40
// DateOnly     2001-09-09
// TimeOnly     02:46:40
// --- 4 2001-09-08T16:01:10.123456-09:45
// Layout       09/08 04:01:10PM '01 -0945
// ANSIC        Sat Sep  8 16:01:10 2001
// UnixDate     Sat Sep  8 16:01:10 NEG 2001
// RubyDate     Sat Sep 08 16:01:10 -0945 2001
// RFC822       08 Sep 01 16:01 NEG
// RFC822Z      08 Sep 01 16:01 -0945
// RFC850       Saturday, 08-Sep-01 16:01:10 NEG
// RFC1123      Sat, 08 Sep 2001 16:01:10 NEG
// RFC1123Z     Sat, 08 Sep 2001 16:01:10 -0945
// RFC3339      2001-09-08T16:01:10-09:45
// RFC3339Nano  2001-09-08T16:01:10.123456-09:45
// Kitchen      4:01PM
// Stamp        Sep  8 16:01:10
// StampMilli   Sep  8 16:01:10.123
// StampMicro   Sep  8 16:01:10.123456
// StampNano    Sep  8 16:01:10.123456000
// DateTime     2001-09-08 16:01:10
// DateOnly     2001-09-08
// TimeOnly     16:01:10
// --- 0 0001-01-01T12:00:00Z
// Layout       01/01 12:00:00PM '01 +0000
// ANSIC        Mon Jan  1 12:00:00 0001
// UnixDate     Mon Jan  1 12:00:00 UTC 0001
// RubyDate     Mon Jan 01 12:00:00 +0000 0001
// RFC822       01 Jan 01 12:00 UTC
// RFC822Z      01 Jan 01 12:00 +0000
// RFC850       Monday, 01-Jan-01 12:00:00 UTC
// RFC1123      Mon, 01 Jan 0001 12:00:00 UTC
// RFC1123Z     Mon, 01 Jan 0001 12:00:00 +0000
// RFC3339      0001-01-01T12:00:00Z
// RFC3339Nano  0001-01-01T12:00:00Z
// Kitchen      12:00PM
// Stamp        Jan  1 12:00:00
// StampMilli   Jan  1 12:00:00.000
// StampMicro   Jan  1 12:00:00.000000
// StampNano    Jan  1 12:00:00.000000000
// DateTime     0001-01-01 12:00:00
// DateOnly     0001-01-01
// TimeOnly     12:00:00
// --- 1 0001-01-01T07:00:00-05:00
// Layout       01/01 07:00:00AM '01 -0500
// ANSIC        Mon Jan  1 07:00:00 0001
// UnixDate     Mon Jan  1 07:00:00 EST 0001
// RubyDate     Mon Jan 01 07:00:00 -0500 0001
// RFC822       01 Jan 01 07:00 EST
// RFC822Z      01 Jan 01 07:00 -0500
// RFC850       Monday, 01-Jan-01 07:00:00 EST
// RFC1123      Mon, 01 Jan 0001 07:00:00 EST
// RFC1123Z     Mon, 01 Jan 0001 07:00:00 -0500
// RFC3339      0001-01-01T07:00:00-05:00
// RFC3339Nano  0001-01-01T07:00:00-05:00
// Kitchen      7:00AM
// Stamp        Jan  1 07:00:00
// StampMilli   Jan  1 07:00:00.000
// StampMicro   Jan  1 07:00:00.000000
// StampNano    Jan  1 07:00:00.000000000
// DateTime     0001-01-01 07:00:00
// DateOnly     0001-01-01
// TimeOnly     07:00:00
// --- 2 0001-01-01T17:30:00+05:30
// Layout       01/01 05:30:00PM '01 +0530
// ANSIC        Mon Jan  1 17:30:00 0001
// UnixDate     Mon Jan  1 17:30:00 IST 0001
// RubyDate     Mon Jan 01 17:30:00 +0530 0001
// RFC822       01 Jan 01 17:30 IST
// RFC822Z      01 Jan 01 17:30 +0530
// RFC850       Monday, 01-Jan-01 17:30:00 IST
// RFC1123      Mon, 01 Jan 0001 17:30:00 IST
// RFC1123Z     Mon, 01 Jan 0001 17:30:00 +0530
// RFC3339      0001-01-01T17:30:00+05:30
// RFC3339Nano  0001-01-01T17:30:00+05:30
// Kitchen      5:30PM
// Stamp        Jan  1 17:30:00
// StampMilli   Jan  1 17:30:00.000
// StampMicro   Jan  1 17:30:00.000000
// StampNano    Jan  1 17:30:00.000000000
// DateTime     0001-01-01 17:30:00
// DateOnly     0001-01-01
// TimeOnly     17:30:00
// --- 3 0001-01-01T13:00:00+01:00
// Layout       01/01 01:00:00PM '01 +0100
// ANSIC        Mon Jan  1 13:00:00 0001
// UnixDate     Mon Jan  1 13:00:00 +0100 0001
// RubyDate     Mon Jan 01 13:00:00 +0100 0001
// RFC822       01 Jan 01 13:00 +0100
// RFC822Z      01 Jan 01 13:00 +0100
// RFC850       Monday, 01-Jan-01 13:00:00 +0100
// RFC1123      Mon, 01 Jan 0001 13:00:00 +0100
// RFC1123Z     Mon, 01 Jan 0001 13:00:00 +0100
// RFC3339      0001-01-01T13:00:00+01:00
// RFC3339Nano  0001-01-01T13:00:00+01:00
// Kitchen      1:00PM
// Stamp        Jan  1 13:00:00
// StampMilli   Jan  1 13:00:00.000
// StampMicro   Jan  1 13:00:00.000000
// StampNano    Jan  1 13:00:00.000000000
// DateTime     0001-01-01 13:00:00
// DateOnly     0001-01-01
// TimeOnly     13:00:00
// --- 4 0001-01-01T02:14:30-09:45
// Layout       01/01 02:14:30AM '01 -0945
// ANSIC        Mon Jan  1 02:14:30 0001
// UnixDate     Mon Jan  1 02:14:30 NEG 0001
// RubyDate     Mon Jan 01 02:14:30 -0945 0001
// RFC822       01 Jan 01 02:14 NEG
// RFC822Z      01 Jan 01 02:14 -0945
// RFC850       Monday, 01-Jan-01 02:14:30 NEG
// RFC1123      Mon, 01 Jan 0001 02:14:30 NEG
// RFC1123Z     Mon, 01 Jan 0001 02:14:30 -0945
// RFC3339      0001-01-01T02:14:30-09:45
// RFC3339Nano  0001-01-01T02:14:30-09:45
// Kitchen      2:14AM
// Stamp        Jan  1 02:14:30
// StampMilli   Jan  1 02:14:30.000
// StampMicro   Jan  1 02:14:30.000000
// StampNano    Jan  1 02:14:30.000000000
// DateTime     0001-01-01 02:14:30
// DateOnly     0001-01-01
// TimeOnly     02:14:30
