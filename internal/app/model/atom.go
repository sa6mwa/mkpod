package model

import (
	"os"
	"path/filepath"
	"strings"
)

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

// Atom_ContainsEpisode returns the index of episode uid in the
// Episodes slice based on UID or -1 if UID does not exist.
func (a *Atom) ContainsEpisode(uid int64) int64 {
	for idx := range a.Episodes {
		if a.Episodes[idx].UID == uid {
			return int64(uid)
		}
	}
	return -1
}

func (a *Atom) LocalStorageDirExpanded() string {
	return resolvetilde(a.Config.LocalStorageDir)
}

func (a *Atom) LamepathExpanded() string {
	return resolvetilde(a.Encoding.Lamepath)
}

func (a *Atom) FFmpegPathExpanded() string {
	return resolvetilde(a.Encoding.FFmpegPath)
}

// resolvetilde returns path where initial tilde (~) is replaced by
// os.UserHomeDir().
func resolvetilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		dirname, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		return filepath.Join(dirname, path[2:])
	}
	return path
}
