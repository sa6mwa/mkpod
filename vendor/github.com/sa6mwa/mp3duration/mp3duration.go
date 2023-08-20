package mp3duration

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/tcolgate/mp3"
)

type Info struct {
	Name         string
	ModTime      time.Time
	Duration     string
	TimeDuration time.Duration
	Seconds      float64
	SecondsInt   int
	Length       int64
	Frames       int
}

// Formats time.Duration according to the duration field of the Itunes Podcast
// document type definition (hh:mm:ss).
func FormatDuration(duration time.Duration) string {
	duration = duration.Round(time.Second)
	hour := duration / time.Hour
	duration -= hour * time.Hour
	minute := duration / time.Minute
	duration -= minute * time.Minute
	second := duration / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
}

// Read duration and other information from a file pointer (*os.File, usually a
// file opened with os.Open()). Returns mp3duration.Info or error.
func Read(f *os.File) (duration Info, err error) {
	var frame mp3.Frame
	if f == nil {
		return duration, errors.New("empty os.File pointer")
	}
	fi, err := f.Stat()
	if err != nil {
		return
	}
	duration.Length = fi.Size()
	duration.Name = fi.Name()
	duration.ModTime = fi.ModTime()
	decoder := mp3.NewDecoder(f)
	skipped := 0
	for {
		err = decoder.Decode(&frame, &skipped)
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return
		}
		duration.TimeDuration = duration.TimeDuration + frame.Duration()
		duration.Frames++
	}
	duration.Seconds = duration.TimeDuration.Seconds()
	duration.SecondsInt = int(math.Round(duration.Seconds))
	duration.Duration = FormatDuration(duration.TimeDuration)
	return
}

// Open and read duration and other information from MP3 file. Closes the file
// after reading it. Returns mp3duration.Info or error.
func ReadFile(mp3file string) (duration Info, err error) {
	f, err := os.Open(mp3file)
	if err != nil {
		return
	}
	defer f.Close()
	duration, err = Read(f)
	return
}
