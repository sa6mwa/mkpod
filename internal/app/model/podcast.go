package model

import (
	"os"
	"path/filepath"
	"strings"
)

type Podcast struct {
	Config        Config         `yaml:"config"`
	FeedFile      string         `yaml:"atom"`
	Title         string         `yaml:"title"`
	Link          string         `yaml:"link"`
	PubDate       ItunesTime     `yaml:"pubDate,omitempty"`
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
	Explicit      ItunesExplicit `yaml:"explicit,omitempty"`
	Keywords      string         `yaml:"keywords"`
	Categories    []Category     `yaml:"categories"`
	Encoding      struct {
		// default is mp3. m4a or m4b means ffmpeg will be used.
		PreferredFormat string `yaml:"preferredFormat,omitempty"`
		Bitrate         int    `yaml:"bitrate"`
		Lamepath        string `yaml:"lamepath"`
		FFmpegPath      string `yaml:"ffmpegpath"`
		CRF             int    `yaml:"crf"`
		ABR             string `yaml:"abr"`
		Coverfront      string `yaml:"coverfront"`
		Genre           string `yaml:"genre"`
		Language        string `yaml:"language"`
	} `yaml:"encoding"`
	Episodes []Episode `yaml:"episodes"`
}

// ContainsEpisode returns the index of episode uid in the
// Episodes slice based on UID or -1 if UID does not exist.
func (p *Podcast) ContainsEpisode(uid int64) int64 {
	for idx := range p.Episodes {
		if p.Episodes[idx].UID == uid {
			return int64(idx)
		}
	}
	return -1
}

func (p *Podcast) LocalStorageDirExpanded() string {
	return resolvetilde(p.Config.LocalStorageDir)
}

func (p *Podcast) LamepathExpanded() string {
	return resolvetilde(p.Encoding.Lamepath)
}

func (p *Podcast) FFmpegPathExpanded() string {
	return resolvetilde(p.Encoding.FFmpegPath)
}

func (p *Podcast) FeedFilePath() string {
	feedFile := strings.TrimSpace(p.FeedFile)
	if feedFile == "" {
		return ""
	}
	feedFile = resolvetilde(feedFile)
	if filepath.IsAbs(feedFile) || strings.TrimSpace(p.LocalStorageDirExpanded()) == "" {
		return feedFile
	}
	return filepath.Join(p.LocalStorageDirExpanded(), feedFile)
}

// resolvetilde returns path where initial tilde (~) is replaced by
// os.UserHomeDir().
func resolvetilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		dirname, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(dirname, path[2:])
	}
	return path
}
