package xmldatetime

import (
	"bytes"
	"encoding/xml"
	"testing"
	"time"
)

type ParseFunc func(string) (time.Time, error)

func cutOut(s string, i int) string {
	if i == 0 {
		return s[1:]
	} else if i == len(s)-1 {
		return s[:i]
	}
	return s[:i] + s[i+1:]
}

func TestParseIncorrect(t *testing.T) {
	fullS := "2017-08-16T13:07:00.1+02:00"
	for _, f := range []ParseFunc{Parse, ParseRe, ParseRe2} {
		for i := 0; i < len(fullS); i++ {
			v := cutOut(fullS, i)
			_, err := f(v)
			if err == nil {
				t.Errorf("want error, got nil: %s", v)
				t.FailNow()
			}
		}
	}
}

func TestParse(t *testing.T) {
	for _, f := range []ParseFunc{Parse, ParseRe, ParseRe2} {
		for _, v := range []string{
			"2017-08-16T13:07:00.09251+02:00",
			"2017-08-16T11:07:00.09251Z",
			"2017-08-16T11:07:00.09251",
		} {
			tm, err := f(v)
			if err != nil {
				t.Errorf("error: %s", err)
				t.FailNow()
			}

			ex := time.Date(2017, time.August, 16, 11, 07, 0, 92510000, time.UTC)
			if !tm.UTC().Equal(ex.UTC()) {
				t.Errorf("want: %v, got: %s", ex, tm)
			}
		}
	}

	// error "2017-08-16T11:07:00.092510",
}

func TestStringify(t *testing.T) {
	ex := time.Date(2017, time.August, 16, 11, 07, 0, 92510000, time.UTC)
	if s := stringify(ex); s != "2017-08-16T11:07:00.09251" {
		t.Errorf("want: 2017-08-16T11:07:00.09251, got: %s", s)
		t.FailNow()
	}

	ex = time.Date(2017, time.August, 16, 11, 07, 0, 92510000, time.FixedZone("-02:00", -2*60*60))
	if s := stringify(ex); s != "2017-08-16T11:07:00.09251-02:00" {
		t.Errorf("want: 2017-08-16T11:07:00.09251-02:00, got: %s", s)
		t.FailNow()
	}

	ex = time.Date(2017, time.August, 16, 11, 07, 0, 0, time.FixedZone("-02:00", -2*60*60))
	if s := stringify(ex); s != "2017-08-16T11:07:00-02:00" {
		t.Errorf("want: 2017-08-16T11:07:00-02:00, got: %s", s)
		t.FailNow()
	}
}

func TestCustomTime_MarshalXML(t *testing.T) {
	ex := time.Date(2017, time.August, 16, 13, 07, 0, 92510000, time.FixedZone("+02:00", 2*60*60))
	c := CustomTime{ex}
	got, err := xml.Marshal(c)
	if err != nil {
		t.Errorf("marshaling: %s", err)
		t.FailNow()
	}
	want := `<CustomTime>2017-08-16T13:07:00.09251+02:00</CustomTime>`

	if !bytes.Equal([]byte(want), got) {
		t.Errorf("want: %v, got: %s", want, got)
		t.FailNow()
	}
}

func TestCustomTime_UnmarshalXML(t *testing.T) {
	xmlS := `<someTime>2017-08-16T13:07:00.09251+02:00</someTime>`
	c := CustomTime{}
	err := xml.Unmarshal([]byte(xmlS), &c)
	if err != nil {
		t.Errorf("problem with unmarshal: %s", err)
	}
	ex := time.Date(2017, time.August, 16, 13, 07, 0, 92510000, time.FixedZone("+02:00", 2*60*60))
	if !c.Time.Equal(ex) {
		t.Errorf("want: %s, got: %s", ex, c.Time)
		t.FailNow()
	}
	if !c.Time.UTC().Equal(ex.UTC()) {
		t.Errorf("want: %s, got: %s", ex.UTC(), c.Time.UTC())
		t.FailNow()
	}
}

func BenchmarkParse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Parse("2017-08-16T13:07:00.09251+02:00")
	}
}

func BenchmarkParseRe(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseRe("2017-08-16T13:07:00.09251+02:00")
	}
}

func BenchmarkParseRe2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseRe2("2017-08-16T13:07:00.09251+02:00")
	}
}
