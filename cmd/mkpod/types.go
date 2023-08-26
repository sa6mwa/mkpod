package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/sa6mwa/mp3duration"
	"gopkg.in/yaml.v2"
)

type Templates struct {
	Lame         *template.Template
	Ffmpeg       *template.Template
	FfmpegToLame *template.Template
}

type AwsHandler struct {
	Session *session.Session
	S3      *s3.S3
}

// Initiate a new AWS session based on properties in private.yaml config file
// (loadConfig() is required before calling this function).
func (s *AwsHandler) NewSession() {
	s.Session = session.Must(session.NewSessionWithOptions(session.Options{
		Profile: atom.Config.Aws.Profile,
		Config: aws.Config{
			Region: aws.String(atom.Config.Aws.Region),
		},
	}))
	s.S3 = s3.New(s.Session)
}

// Diff file by downloading from the bucket and compare it to file.
func (s *AwsHandler) Diff(bucket string, key string, file string) error {
	fileContent, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	downloader := s3manager.NewDownloader(s.Session)
	buf := aws.NewWriteAtBuffer([]byte{})
	n, err := downloader.Download(buf, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "NotFound", "NoSuchKey":
				log.Printf("Skipping diff of %s: s3://%s: %v", file, path.Join(bucket, key), err)
				return nil
			default:
				return err
			}
		} else {
			return err
		}
	}
	log.Printf("Downloaded %d bytes from s3://%s into buffer", n, path.Join(bucket, key))

	log.Printf("Diff between %s and s3://%s follows...", file, path.Join(bucket, key))
	edits := myers.ComputeEdits(span.URIFromPath("s3://"+path.Join(bucket, key)), string(buf.Bytes()), string(fileContent))
	diff := fmt.Sprint(gotextdiff.ToUnified("s3://"+path.Join(bucket, key), file, string(buf.Bytes()), edits))
	fmt.Println(diff)

	return nil
}

// Upload file as key to S3 bucket.
func (s *AwsHandler) Upload(bucket string, key string, contentType string, file string) error {
	log.Printf("Uploading %s to s3://%s", file, path.Join(bucket, key))
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	uploader := s3manager.NewUploader(s.Session)
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		Body:        f,
	})
	if err != nil {
		return err
	}
	log.Printf("Uploaded %s", aws.StringValue(&result.Location))
	return nil
}

// Download key from S3 bucket and store key as file under localStorageDir
// property in private.yaml (running loadConfig() prior to calling this function
// is required).
func (s *AwsHandler) Download(bucket string, key string) error {
	log.Printf("Downloading s3://%s to %s", path.Join(bucket, key), path.Join(atom.LocalStorageDirExpanded(), key))
	downloader := s3manager.NewDownloader(s.Session)
	completePath := path.Join(atom.LocalStorageDirExpanded(), key)
	dirPath := path.Dir(completePath)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return err
	}
	var f *os.File
	fi, err := os.Stat(completePath)
	if err != nil {
		// does file exist? if not, create it
		if errors.Is(err, fs.ErrNotExist) {
			f, err = os.Create(completePath)
			if err != nil {
				return err
			}
			defer f.Close()
		} else {
			// error is something else
			return err
		}
	} else {
		// No error, could stat file
		// Get content length of file in S3 bucket and compare size to file already
		// on disk, do not download if they match.
		size, err := awsHandler.GetSize(bucket, key)
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				switch awsErr.Code() {
				case "NotFound", "NoSuchKey":
					log.Printf("s3://%s does not exist, will use local file %s only", path.Join(bucket, key), completePath)
					return nil
				default:
					return err
				}
			} else {
				return err
			}
		}
		if size != fi.Size() {
			// size does not match, download file (truncate file and fall through)
			f, err = os.Create(completePath)
			if err != nil {
				return err
			}
			defer f.Close()
		} else {
			log.Printf("Will not download %s as local file size and content length of s3://%s match", completePath, path.Join(bucket, key))
			return nil
		}
	}
	n, err := downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	log.Printf("Downloaded %d bytes from s3://%s to %s", n, path.Join(bucket, key), completePath)
	return nil
}

func (s *AwsHandler) GetSize(bucket string, key string) (int64, error) {
	result, err := s.S3.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, err
	}
	return aws.Int64Value(result.ContentLength), nil
}

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

type Config struct {
	BaseURL         string    `yaml:"baseURL"`
	Image           string    `yaml:"image"`
	DefaultPodImage string    `yaml:"defaultPodImage"`
	Aws             AwsConfig `yaml:"aws"`
	LocalStorageDir string    `yaml:"localStorageDir"`
}

func (c *Config) LocalStorageDirExpanded() string {
	return resolvetilde(c.LocalStorageDir)
}

type AwsConfig struct {
	Profile string  `yaml:"profile"`
	Region  string  `yaml:"region"`
	Buckets Buckets `yaml:"buckets"`
}

type Buckets struct {
	Input  string `yaml:"input"`
	Output string `yaml:"output"`
}

type Atom struct {
	Config        Config         `yaml:"config"`
	Atom          string         `yaml:"atom"`
	Title         string         `yaml:"title"`
	Link          string         `yaml:"link"`
	PubDate       ItunesTime     `yaml:"pubDate"`
	LastBuildDate ItunesTime     `yaml:"lastBuildDate"`
	TTL           int            `yaml:"ttl"`
	Language      string         `yaml:"language"`
	Copyright     string         `yaml:"copyright"`
	WebMaster     string         `yaml:"webMaster"`
	Description   string         `yaml:"description"`
	Subtitle      string         `yaml:"subtitle"`
	OwnerName     string         `yaml:"ownerName"`
	OwnerEmail    string         `yaml:"ownerEmail"`
	Author        string         `yaml:"author"`
	Explicit      ItunesExplicit `yaml:"explicit"`
	Keywords      string         `yaml:"keywords"`
	Category      string         `yaml:"category"`
	Encoding      struct {
		Bitrate    int    `yaml:"bitrate"`
		Lamepath   string `yaml:"lamepath"`
		Ffmpegpath string `yaml:"ffmpegpath"`
		CRF        int    `yaml:"crf"`
		ABR        string `yaml:"abr"`
		Coverfront string `yaml:"coverfront"`
		Genre      string `yaml:"genre"`
		Language   string `yaml:"language"`
	} `yaml:"encoding"`
	Episodes []Episode `yaml:"episodes"`
}

type Combined struct {
	Atom    *Atom
	Episode *Episode
}

func (a *Atom) LocalStorageDirExpanded() string {
	return a.Config.LocalStorageDirExpanded()
}
func (a *Atom) LamepathExpanded() string {
	return resolvetilde(a.Encoding.Lamepath)
}
func (a *Atom) FfmpegpathExpanded() string {
	return resolvetilde(a.Encoding.Ffmpegpath)
}

// Returns index of episode in Episodes slice based on UID or -1 if UID does not
// exist.
func (a *Atom) ContainsEpisode(uid int64) int {
	for idx := range a.Episodes {
		if a.Episodes[idx].UID == uid {
			return idx
		}
	}
	return -1
}

func (a *Atom) YamlString() (marshalledString string, err error) {
	var marshalledBytes []byte
	marshalledBytes, err = a.Yaml()
	if err != nil {
		return
	}
	marshalledString = string(marshalledBytes)
	return
}

func (a *Atom) Yaml() ([]byte, error) {
	return yaml.Marshal(a)
}

type Defaults struct {
	Bitrate int `yaml:"bitrate"`
}

type Episode struct {
	UID         int64          `yaml:"uid"`
	Title       string         `yaml:"title"`
	PubDate     ItunesTime     `yaml:"pubDate"`
	Link        string         `yaml:"link"`
	Duration    ItunesDuration `yaml:"duration"`
	Author      string         `yaml:"author"`
	Explicit    ItunesExplicit `yaml:"explicit"`
	Subtitle    string         `yaml:"subtitle"`
	Description string         `yaml:"description"`
	Type        string         `yaml:"type"`
	Length      int64          `yaml:"length"`
	Image       string         `yaml:"image"`
	Input       string         `yaml:"input"`
	Output      string         `yaml:"output"`
	Format      string         `yaml:"format"`
}

type Rss struct {
	XMLName xml.Name `xml:"rss"`
	Text    string   `xml:",chardata"`
	Version string   `xml:"version,attr"`
	Itunes  string   `xml:"itunes,attr"`
	Atom    string   `xml:"atom,attr"`
	Channel struct {
		Text string `xml:",chardata"`
		Link []struct {
			Text string `xml:",chardata"`
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
			Type string `xml:"type,attr"`
		} `xml:"link"`
		Title         string `xml:"title"`
		PubDate       string `xml:"pubDate"`
		LastBuildDate string `xml:"lastBuildDate"`
		Ttl           int    `xml:"ttl"`
		Language      string `xml:"language"`
		Copyright     string `xml:"copyright"`
		WebMaster     string `xml:"webMaster"`
		Description   string `xml:"description"`
		Subtitle      string `xml:"subtitle"`
		Owner         struct {
			Text  string `xml:",chardata"`
			Name  string `xml:"name"`
			Email string `xml:"email"`
		} `xml:"owner"`
		Author   string `xml:"author"`
		Explicit string `xml:"explicit"`
		Image    struct {
			Text  string `xml:",chardata"`
			Href  string `xml:"href,attr"`
			URL   string `xml:"url"`
			Title string `xml:"title"`
			Link  string `xml:"link"`
		} `xml:"image"`
		Category struct {
			Text     string `xml:",chardata"`
			AttrText string `xml:"text,attr"`
		} `xml:"category"`
		Item []struct {
			Text string `xml:",chardata"`
			Guid struct {
				Text        string `xml:",chardata"`
				IsPermaLink string `xml:"isPermaLink,attr"`
			} `xml:"guid"`
			Title       string `xml:"title"`
			PubDate     string `xml:"pubDate"`
			Link        string `xml:"link"`
			Duration    string `xml:"duration"`
			Author      string `xml:"author"`
			Explicit    string `xml:"explicit"`
			Summary     string `xml:"summary"`
			Subtitle    string `xml:"subtitle"`
			Description string `xml:"description"`
			Enclosure   struct {
				Text   string `xml:",chardata"`
				Type   string `xml:"type,attr"`
				URL    string `xml:"url,attr"`
				Length int64  `xml:"length,attr"`
			} `xml:"enclosure"`
			Image struct {
				Text string `xml:",chardata"`
				Href string `xml:"href,attr"`
			} `xml:"image"`
		} `xml:"item"`
	} `xml:"channel"`
}
