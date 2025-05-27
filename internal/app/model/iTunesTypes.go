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
		newt, err = time.Parse(time.RFC1123Z, timeString)
		if err != nil {
			return err
		}
	}
	t.Time = newt
	return nil
}

// Custom marshal function to write time.Time as RFC1123Z (Itunes "RFC2822" time format).
func (t ItunesTime) MarshalYAML() (interface{}, error) {
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
func (d ItunesDuration) MarshalYAML() (interface{}, error) {
	return mp3duration.FormatDuration(d.Duration), nil
}

// Return duration as string in Itunes Duration HH:MM:SS format.
func (d ItunesDuration) String() string {
	return mp3duration.FormatDuration(d.Duration)
}
