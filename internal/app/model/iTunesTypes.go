package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sa6mwa/mp3duration"
)

type ItunesTime struct {
	time.Time
}

// Custom unmarshal function for RFC1123Z time (Itunes "RFC2822" date format).
func (t *ItunesTime) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var buf string
	err := unmarshal(&buf)
	if err != nil {
		return err
	}

	timeString := strings.TrimSpace(buf)
	var newt time.Time

	switch strings.ToLower(timeString) {
	case "", "today", "now":
		newt = time.Now().UTC()
	default:
		newt, err = parseItunesTimeString(timeString, time.Now().UTC())
		if err != nil {
			return err
		}
		if newt.IsZero() {
			newt = time.Now().UTC()
		}
	}
	t.Time = newt
	return nil
}

func parseItunesTimeString(input string, now time.Time) (time.Time, error) {
	normalized := strings.ToLower(strings.TrimSpace(input))
	switch normalized {
	case "", "today", "now":
		return now.UTC(), nil
	case "yesterday":
		return startOfRelativeDay(now, -1), nil
	}

	if relative, ok := parseRelativeTime(normalized, now); ok {
		return relative, nil
	}

	parsed, err := time.Parse(time.RFC1123Z, input)
	if err != nil {
		return time.Time{}, err
	}
	if parsed.IsZero() {
		return now.UTC(), nil
	}
	return parsed, nil
}

func parseRelativeTime(input string, now time.Time) (time.Time, bool) {
	fields := strings.Fields(input)
	if len(fields) == 0 || len(fields) > 2 {
		return time.Time{}, false
	}

	baseDay := startOfRelativeDay(now, 0)
	switch fields[0] {
	case "today", "now":
		if len(fields) == 1 {
			return now.UTC(), true
		}
		fields = fields[1:]
	case "yesterday":
		baseDay = startOfRelativeDay(now, -1)
		if len(fields) == 1 {
			return baseDay, true
		}
		fields = fields[1:]
	}

	if len(fields) != 1 {
		return time.Time{}, false
	}

	hour, min, sec, ok := parseClock(fields[0])
	if !ok {
		return time.Time{}, false
	}
	return time.Date(baseDay.Year(), baseDay.Month(), baseDay.Day(), hour, min, sec, 0, time.UTC), true
}

func parseClock(value string) (hour, min, sec int, ok bool) {
	switch {
	case len(value) == 4 && strings.IndexByte(value, ':') == -1:
		h, errH := strconv.Atoi(value[:2])
		m, errM := strconv.Atoi(value[2:])
		if errH != nil || errM != nil {
			return 0, 0, 0, false
		}
		return validClock(h, m, 0)
	case strings.Count(value, ":") == 1:
		parts := strings.Split(value, ":")
		h, errH := strconv.Atoi(parts[0])
		m, errM := strconv.Atoi(parts[1])
		if errH != nil || errM != nil {
			return 0, 0, 0, false
		}
		return validClock(h, m, 0)
	case strings.Count(value, ":") == 2:
		parts := strings.Split(value, ":")
		h, errH := strconv.Atoi(parts[0])
		m, errM := strconv.Atoi(parts[1])
		s, errS := strconv.Atoi(parts[2])
		if errH != nil || errM != nil || errS != nil {
			return 0, 0, 0, false
		}
		return validClock(h, m, s)
	default:
		return 0, 0, 0, false
	}
}

func validClock(hour, min, sec int) (int, int, int, bool) {
	if hour < 0 || hour > 23 || min < 0 || min > 59 || sec < 0 || sec > 59 {
		return 0, 0, 0, false
	}
	return hour, min, sec, true
}

func startOfRelativeDay(now time.Time, dayOffset int) time.Time {
	base := now.UTC().AddDate(0, 0, dayOffset)
	return time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, time.UTC)
}

// Custom marshal function to write time.Time as RFC1123Z (Itunes "RFC2822" time format).
// Returns nil for zero times to support omitempty behavior.
func (t ItunesTime) MarshalYAML() (interface{}, error) {
	if t.IsZero() {
		return nil, nil
	}
	return t.Format(time.RFC1123Z), nil
}

// Override default String() function to output time in RFC1123Z format (Itunes "RFC2822" time format).
func (t ItunesTime) String() string {
	return t.Format(time.RFC1123Z)
}

// Output yes or no for the explicit field.
type ItunesExplicit struct {
	S string
}

func (e *ItunesExplicit) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var buf string
	err := unmarshal(&buf)
	if err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(buf)) {
	case "yes", "true":
		e.S = "yes"
	default:
		e.S = "no"
	}
	return nil
}
func (e ItunesExplicit) MarshalYAML() (interface{}, error) {
	if strings.TrimSpace(e.S) == "" {
		return nil, nil
	}
	return e.S, nil
}
func (e ItunesExplicit) String() string {
	return e.S
}

// The Apple RSS has a specific duration format.
type ItunesDuration struct {
	time.Duration
}

func (d *ItunesDuration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var buf string
	err := unmarshal(&buf)
	if err != nil {
		return err
	}

	durationString := strings.TrimSpace(buf)
	switch strings.ToLower(durationString) {
	case "", "gen", "generate", "parse":
		d.Duration = 0
		return nil
	}
	// itunes:duration format is hh:mm:ss
	values := strings.Split(durationString, ":")
	if len(values) != 3 {
		return fmt.Errorf("unmarshal error: duration must be in the format HH:MM:SS, not %s (delete duration to regenerate)", durationString)
	}
	h, err := strconv.Atoi(values[0])
	if err != nil {
		return err
	}
	m, err := strconv.Atoi(values[1])
	if err != nil {
		return err
	}
	s, err := strconv.Atoi(values[2])
	if err != nil {
		return err
	}
	var newd time.Duration
	newd = time.Duration(h) * time.Hour
	newd = newd + (time.Duration(m) * time.Minute)
	newd = newd + (time.Duration(s) * time.Second)
	d.Duration = newd
	return nil
}

// Format duration according to Itunes podcast Atom specification (HH:MM:SS).
// Returns nil for zero durations to support omitempty behavior.
func (d ItunesDuration) MarshalYAML() (interface{}, error) {
	if d.Duration == 0 {
		return nil, nil
	}
	return mp3duration.FormatDuration(d.Duration), nil
}

// Return duration as string in Itunes Duration HH:MM:SS format.
func (d ItunesDuration) String() string {
	return mp3duration.FormatDuration(d.Duration)
}
