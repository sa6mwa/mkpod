package model

import (
	"encoding/json"
	"time"
)

type FFprobeDuration struct {
	time.Duration
}

func (d *FFprobeDuration) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}
	duration, err := time.ParseDuration(str + "s")
	if err != nil {
		return err
	}
	d.Duration = duration
	return nil
}

type FFprobeJSON struct {
	Format struct {
		Filename   string          `json:"filename"`
		NbStreams  int             `json:"nb_streams"`
		NbPrograms int             `json:"nb_programs"`
		FormatName string          `json:"format_name"`
		StartTime  string          `json:"start_time"`
		Duration   FFprobeDuration `json:"duration"`
		Size       string          `json:"size"`
		BitRate    string          `json:"bit_rate"`
		ProbeScore int             `json:"probe_score"`
		Tags       struct {
			MajorBrand       string `json:"major_brand"`
			MinorVersion     string `json:"minor_version"`
			CompatibleBrands string `json:"compatible_brands"`
			Title            string `json:"title"`
			Artist           string `json:"artist"`
			Album            string `json:"album"`
			Encoder          string `json:"encoder"`
			ITunSMPB         string `json:"iTunSMPB"`
		} `json:"tags"`
	} `json:"format"`
}
