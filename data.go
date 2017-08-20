package xmldatetime

import (
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type CustomTime struct {
	time.Time
}

func exactInt(s string, l int) (int, string, error) {
	if len(s) < l {
		return 0, s, errors.New("not enough")
	}
	i, err := strconv.ParseInt(s[:l], 10, 64)
	return int(i), s[l:], err
}

var not time.Time

// Parses implements https://www.w3.org/TR/xmlschema-2 # 3.2.7.1 Lexical representation (dateTime)
// '-'? yyyy '-' mm '-' dd 'T' hh ':' mm ':' ss ('.' s+)? (zzzzzz)?
// (('+' | '-') hh ':' mm) | 'Z'
// It's worth to mention that this is the fastest implementation of parser:
//  $go test -bench .
//  BenchmarkParse-4      	 5000000	       335 ns/op
//  BenchmarkParseRe-4    	 1000000	      1715 ns/op
//  BenchmarkParseRe2-4   	 1000000	      1686 ns/op
//  PASS
//  ok  	doz.pl/companions/data	6.298s
func Parse(s string) (time.Time, error) {
	sign := 1
	if s[0] == '-' {
		sign = -1
		s = s[1:]
	} else if s[0] == '+' {
		return not, errors.New("+ before year not allowed")
	}
	year, s, err := exactInt(s, 4)
	if err != nil {
		return not, err
	}
	year *= sign
	if s[0] != '-' {
		return not, errors.New("expected - in dateTime format after 4 digit year")
	}
	s = s[1:]

	month, s, err := exactInt(s, 2)
	if err != nil {
		return not, err
	}
	if s[0] != '-' {
		return not, errors.New("expected - in dateTime format after 2 digit month")
	}
	s = s[1:]

	day, s, err := exactInt(s, 2)
	if err != nil {
		return not, err
	}
	if s[0] != 'T' {
		return not, errors.New("expected T in dateTime format")
	}
	s = s[1:]

	hour, s, err := exactInt(s, 2)
	if err != nil {
		return not, err
	}
	if s[0] != ':' {
		return not, errors.New("expected : in dateTime format after 2 digit hour")
	}
	s = s[1:]

	minute, s, err := exactInt(s, 2)
	if err != nil {
		return not, err
	}
	if s[0] != ':' {
		return not, errors.New("expected : in dateTime format after 2 digit minute")
	}
	s = s[1:]

	second, s, err := exactInt(s, 2)
	if err != nil {
		return not, err
	}
	nsec := 0
	if len(s) > 0 && s[0] == '.' {
		nsec, s, err = parseFractionalSecond(s[1:])
		if err != nil {
			return not, err
		}
	}
	loc, err := parseTz(s)
	if err != nil {
		return not, err
	}

	return time.Date(year, time.Month(month), day, hour, minute, second, nsec, loc), nil
}

func parseFractionalSecond(s string) (int, string, error) {
	i := 0
	lastDigit := 0
	var nsec int
	for ; i < len(s) && i < 10 && '0' <= s[i] && s[i] <= '9'; i++ {
		nsec *= 10
		lastDigit = int(s[i] - '0')
		nsec = nsec + lastDigit
	}
	if i == 0 {
		return nsec, s, errors.New("after . indicating fractional seconds there must be digit")
	}
	if lastDigit == 0 {
		// https://www.w3.org/TR/xmlschema-2/#dateTime
		// 3.2.7.2 Canonical representation
		// The fractional second string, if present, must not end in '0';
		return nsec, s, errors.New("fractional second must not end in '0'")
	}
	s = s[i:]
	if i > 9 {
		//nsec = nsec / int(math.Pow10(i-9))
		return nsec, s, errors.New("does not support fraction with precision smaller than 1e-9")
	} else if i < 9 {
		nsec = nsec * int(math.Pow10(9-i))
	}
	return nsec, s, nil
}

var (
	xmlDateTimeRe = regexp.MustCompile(
		`^(?P<year>-?\d{4})-(?P<month>\d{2})-(?P<day>\d{2})T(?P<hour>\d{2}):(?P<min>\d{2}):(?P<sec>\d{2})` +
			`(?:\.(?P<frac>\d+))?(?:(?:(?P<tzh>[+-]\d{2}):(?P<tzm>\d{2}))|Z)?$`)
)

func ParseRe(s string) (time.Time, error) {
	sub := xmlDateTimeRe.FindStringSubmatch(s)
	if len(sub) == 0 {
		return not, errors.New("does not match format")
	}
	res := struct {
		year, month, day, hour, minute, second, nsecond int
		loc                                             *time.Location
	}{loc: time.UTC}
	consumed := 1
	for _, v := range []struct {
		val *int
		l   int
	}{
		{&res.year, 4}, {&res.month, 2}, {&res.day, 2},
		{&res.hour, 2}, {&res.minute, 2}, {&res.second, 2},
	} {
		w, err := strconv.ParseInt(sub[consumed], 10, 64)
		if err != nil {
			return not, err
		}
		*v.val = int(w)
		consumed++
	}
	if sub[consumed] != "" {
		nsec, _, err := parseFractionalSecond(sub[7])
		if err != nil {
			return not, err
		}
		res.nsecond = nsec
	}
	consumed++
	if len(sub) >= 9 && (sub[8] != "" || sub[9] != "") {
		tmh, err := strconv.ParseInt(sub[8], 10, 32)
		if err != nil {
			return not, errors.New("cannot parse zone hour")
		}
		if tmh > 14 {
			return not, errors.New("max timezone hour is 14")
		}
		tmm, err := strconv.ParseInt(sub[9], 10, 32)
		if err != nil {
			return not, errors.New("cannot parse zone minute")
		}
		res.loc = time.FixedZone("", ((int(tmh)*60)+int(tmm))*60)
	}
	return time.Date(res.year, time.Month(res.month), res.day,
		res.hour, res.minute, res.second, res.nsecond, res.loc), nil
}

func ParseRe2(s string) (time.Time, error) {
	sub := xmlDateTimeRe.FindStringSubmatch(s)
	if len(sub) == 0 {
		return not, errors.New("does not match format")
	}
	res := struct {
		year, month, day, hour, minute, second, nsecond int
		loc                                             *time.Location
	}{loc: time.UTC}
	{
		w, err := strconv.ParseInt(sub[1], 10, 64)
		if err != nil {
			return not, err
		}
		res.year = int(w)
	}
	{
		w, err := strconv.ParseInt(sub[2], 10, 64)
		if err != nil {
			return not, err
		}
		res.month = int(w)
	}
	{
		w, err := strconv.ParseInt(sub[3], 10, 64)
		if err != nil {
			return not, err
		}
		res.day = int(w)
	}
	{
		w, err := strconv.ParseInt(sub[4], 10, 64)
		if err != nil {
			return not, err
		}
		res.hour = int(w)
	}
	{
		w, err := strconv.ParseInt(sub[5], 10, 64)
		if err != nil {
			return not, err
		}
		res.minute = int(w)
	}
	{
		w, err := strconv.ParseInt(sub[6], 10, 64)
		if err != nil {
			return not, err
		}
		res.second = int(w)
	}

	if sub[7] != "" {
		nsec, _, err := parseFractionalSecond(sub[7])
		if err != nil {
			return not, err
		}
		res.nsecond = nsec
	}
	if len(sub) >= 8 && (sub[8] != "" || sub[9] != "") {
		tmh, err := strconv.ParseInt(sub[8], 10, 32)
		if err != nil {
			return not, errors.New("cannot parse zone hour")
		}
		if tmh > 14 {
			return not, errors.New("max timezone hour is 14")
		}
		tmm, err := strconv.ParseInt(sub[9], 10, 32)
		if err != nil {
			return not, errors.New("cannot parse zone minute")
		}
		res.loc = time.FixedZone("", ((int(tmh)*60)+int(tmm))*60)
	}
	return time.Date(res.year, time.Month(res.month), res.day,
		res.hour, res.minute, res.second, res.nsecond, res.loc), nil
}

func parseTz(s string) (*time.Location, error) {
	loc := time.UTC
	switch len(s) {
	case 0:
	case 1:
		if s[0] != 'Z' {
			return nil, errors.New("tz 1 char but not Z")
		}
	case 6:
		tz := s
		sign := 0
		switch s[0] {
		case '+':
			sign = 1
		case '-':
			sign = -1
		default:
			return nil, errors.New("timezone must start from + or -")
		}
		s = s[1:]

		hz, s, err := exactInt(s, 2)
		if err != nil {
			return nil, err
		}
		if hz > 14 {
			return nil, errors.New("max timezone hour is 14")
		}
		if s[0] != ':' {
			return nil, errors.New("expected : in dateTime format after 2 digit timezone hour")
		}
		s = s[1:]
		mz, s, err := exactInt(s, 2)
		if err != nil {
			return nil, err
		}
		loc = time.FixedZone(tz, sign*((hz*60)+mz)*60)
	default:
		return nil, errors.New("timezone requires exactly 6 characters if not Z")
	}
	return loc, nil
}

func (c *CustomTime) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var v string
	d.DecodeElement(&v, &start)
	t, err := Parse(v)
	if err != nil {
		return err
	}
	c.Time = t
	return nil
}

func stringify(t time.Time) string {
	v := t.Format("2006-01-02T15:04:05")
	if n := t.Nanosecond(); n > 0 {
		v += strings.TrimRight(fmt.Sprintf(".%09d", n), "0")
	}
	if loc := t.Location(); loc != nil {
		_, offset := t.Zone()
		if offset != 0 {
			minutes := offset / 60
			hours := minutes / 60
			if hours > 0 {
				v += "+"
			}
			v += fmt.Sprintf("%03d:%02d", hours, minutes-60*hours)
		}
	}
	return v
}

func (c *CustomTime) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(stringify(c.Time), start)
}
