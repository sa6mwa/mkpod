package spec

var RequiredTopLevelFields = []string{
	"author",
	"config.baseURL",
	"config.image",
	"config.defaultPodImage",
	"atom",
	"title",
	"ttl",
	"language",
	"copyright",
	"webMaster",
	"description",
	"subtitle",
	"ownerName",
	"ownerEmail",
}

var EpisodeFieldsDefaultedFromTopLevel = []string{
	"author",
	"explicit",
	"image",
}
