package model

import "github.com/sa6mwa/id3v24"

type Episode struct {
	UID              int64            `yaml:"uid"`
	Title            string           `yaml:"title"`
	PubDate          ItunesTime       `yaml:"pubDate"`
	Link             string           `yaml:"link"`
	Duration         ItunesDuration   `yaml:"duration"`
	Author           string           `yaml:"author"`
	Explicit         ItunesExplicit   `yaml:"explicit,omitempty"`
	Subtitle         string           `yaml:"subtitle"`
	Description      string           `yaml:"description"`
	Type             string           `yaml:"type"`
	Length           int64            `yaml:"length"`
	Image            string           `yaml:"image"`
	Input            string           `yaml:"input"`
	Output           string           `yaml:"output,omitempty"`
	Format           string           `yaml:"format,omitempty"`
	EncodingLanguage string           `yaml:"encodingLanguage,omitempty"`
	Chapters         []id3v24.Chapter `yaml:"chapters,omitempty"`
}
