package model

import "encoding/xml"

// This struct is not used anywhere as of now. One use case could be to validate the rendered podcast.rss by the ForParsing port and parser adapter.
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
