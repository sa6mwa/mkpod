package model

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
