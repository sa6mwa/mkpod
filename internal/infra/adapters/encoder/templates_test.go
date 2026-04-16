package encoder

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

func TestAACTemplatesUseBuiltInFFmpegEncoder(t *testing.T) {
	values := templateValues{
		Podcast: &model.Podcast{
			Config: model.Config{
				LocalStorageDir: "/tmp/pod",
			},
			Link: "https://example.com/show",
			Encoding: struct {
				PreferredFormat string `yaml:"preferredFormat,omitempty"`
				Bitrate         int    `yaml:"bitrate"`
				Lamepath        string `yaml:"lamepath"`
				FFmpegPath      string `yaml:"ffmpegpath"`
				CRF             int    `yaml:"crf"`
				ABR             string `yaml:"abr"`
				Coverfront      string `yaml:"coverfront"`
				Genre           string `yaml:"genre"`
				Language        string `yaml:"language"`
			}{
				Lamepath:   "lame",
				FFmpegPath: "ffmpeg",
				CRF:        28,
				ABR:        "128k",
				Coverfront: "artwork/cover.jpg",
				Genre:      "Podcast",
				Language:   "eng",
			},
		},
		Episode: &model.Episode{
			UID:    1,
			Title:  "Episode 1",
			Input:  "masters/episode.wav",
			Output: "episode.m4a",
		},
		MetadataFile: "/tmp/mkpod-ffmetadata.txt",
	}

	tests := []struct {
		name     string
		template string
	}{
		{name: "video AAC template", template: ffmpegCommandTemplate},
		{name: "m4a AAC template", template: ffmpegToM4ACommandTemplate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("cmd").Funcs(defaultFuncMap()).Parse(tt.template)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, values); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			command := buf.String()
			if !strings.Contains(command, "-c:a aac") {
				t.Fatalf("command %q does not use built-in AAC encoder", command)
			}
			if strings.Contains(command, "libfdk_aac") {
				t.Fatalf("command %q still references libfdk_aac", command)
			}
		})
	}
}
