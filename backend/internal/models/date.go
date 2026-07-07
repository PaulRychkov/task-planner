package models

import (
	"database/sql/driver"
	"fmt"
	"time"
)

type Date struct {
	Year  int
	Month time.Month
	Day   int
}

func NewDate(year int, month time.Month, day int) Date {
	return DateOf(time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
}

func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, fmt.Errorf("parse date %q: %w", s, err)
	}
	return DateOf(t), nil
}

func (d Date) IsZero() bool {
	return d == Date{}
}

func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day)
}

func (d Date) UTC() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func (d Date) In(loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

func (d Date) AddDays(n int) Date {
	return DateOf(time.Date(d.Year, d.Month, d.Day+n, 0, 0, 0, 0, time.UTC))
}

func (d Date) DaysSince(o Date) int {
	return int(d.UTC().Sub(o.UTC()) / (24 * time.Hour))
}

func (d Date) Weekday() time.Weekday {
	return d.UTC().Weekday()
}

func (d Date) ISOWeekday() int {
	wd := int(d.Weekday())
	if wd == 0 {
		return 7
	}
	return wd
}

func (d Date) Before(o Date) bool {
	if d.Year != o.Year {
		return d.Year < o.Year
	}
	if d.Month != o.Month {
		return d.Month < o.Month
	}
	return d.Day < o.Day
}

func (d Date) After(o Date) bool {
	return o.Before(d)
}

func DaysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

func (d *Date) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" {
		return nil
	}
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return fmt.Errorf("invalid date json %s", s)
	}
	parsed, err := ParseDate(s[1 : len(s)-1])
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d Date) Value() (driver.Value, error) {
	return d.String(), nil
}

func (d *Date) Scan(v any) error {
	switch t := v.(type) {
	case time.Time:
		*d = DateOf(t)
		return nil
	case string:
		parsed, err := ParseDate(t)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case []byte:
		parsed, err := ParseDate(string(t))
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	default:
		return fmt.Errorf("cannot scan %T into Date", v)
	}
}

func (Date) GormDataType() string {
	return "date"
}
