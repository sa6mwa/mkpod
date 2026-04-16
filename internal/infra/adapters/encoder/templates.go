package encoder

import "github.com/sa6mwa/mkpod/internal/app/model"

type templateValues struct {
	Podcast      *model.Podcast
	Episode      *model.Episode
	MetadataFile string
}

var lameCommandTemplate string = `{{ $PRE := "" }}{{ if ne .Podcast.LocalStorageDirExpanded "" }}{{ $PRE = print .Podcast.LocalStorageDirExpanded "/" }}{{ end }}{{ .Podcast.LamepathExpanded }} -b {{ .Podcast.Encoding.Bitrate }} {{ escape (print $PRE .Episode.Input) }} {{ escape (print $PRE .Episode.Output) }}`

var ffmpegCommandTemplate string = `{{ $PRE := ""}}{{ if ne .Podcast.LocalStorageDirExpanded ""}}{{ $PRE = print .Podcast.LocalStorageDirExpanded "/"}}{{ end }}{{ .Podcast.FFmpegPathExpanded }} -y -i {{ escape (print $PRE .Episode.Input) }} -pix_fmt yuv420p -colorspace bt709 -color_trc bt709 -color_primaries bt709 -color_range tv -c:v libx264 -profile:v high -crf {{ .Podcast.Encoding.CRF }} -maxrate 1M -bufsize 2M -preset medium -coder 1 -movflags +faststart -x264-params open-gop=0 -c:a aac -b:a {{ .Podcast.Encoding.ABR }} {{ escape (print $PRE .Episode.Output) }}`

// ffmpeg to Lame
var ffmpegToAudioCommandTemplate string = `{{ $PRE := ""}}{{ if ne .Podcast.LocalStorageDirExpanded ""}}{{ $PRE = print .Podcast.LocalStorageDirExpanded "/"}}{{ end }}{{ .Podcast.FFmpegPathExpanded }} -y -i {{ escape (print $PRE .Episode.Input) }} -vn -f wav -c:a pcm_s16le -ac 2 pipe: | {{ .Podcast.LamepathExpanded }} -b {{ .Podcast.Encoding.Bitrate }} --add-id3v2 --tv TLAN={{ if ne .Episode.EncodingLanguage "" }}{{ escape .Episode.EncodingLanguage }}{{ else }}{{ escape .Podcast.Encoding.Language }}{{ end }} --tt {{ escape .Episode.Title }} --ta {{ escape .Podcast.Author }} --tl {{ escape .Podcast.Title }} --ty {{ escape (.Episode.PubDate.Format "2006") }} --tc {{ escape .Episode.Subtitle }} --tn {{ .Episode.UID }} --tg {{ escape .Podcast.Encoding.Genre }} --ti {{ escape (print $PRE .Podcast.Encoding.Coverfront) }} --tv WOAR={{ escape .Podcast.Link }} - {{ escape (print $PRE .Episode.Output) }}`

// Used to make m4a or m4b audio files. Combines the audio, cover
// image, and metadata with chapters into the output m4a/m4b in a
// single run using ffmpeg's built-in AAC encoder.
var ffmpegToM4ACommandTemplate string = `{{ $PRE := "" }}{{ if ne .Podcast.LocalStorageDirExpanded ""}}{{ $PRE = print .Podcast.LocalStorageDirExpanded "/"}}{{ end }}{{ .Podcast.FFmpegPathExpanded }} -y -i {{ escape (print $PRE .Episode.Input) }} -i {{ escape (print $PRE .Podcast.Encoding.Coverfront) }} -i {{ escape .MetadataFile }} -map 0:a -c:a aac -b:a {{ .Podcast.Encoding.ABR }} -metadata:s:a:0 language={{ if ne .Episode.EncodingLanguage "" }}{{ escape .Episode.EncodingLanguage }}{{ else }}{{ escape .Podcast.Encoding.Language }}{{ end }} -map 1:v -c:v mjpeg -disposition:v:0 attached_pic -metadata:s:v title="Cover" -metadata:s:v comment="Cover (front)" -map_metadata 2 -map_chapters 2 -movflags faststart {{ escape (print $PRE .Episode.Output) }}`
