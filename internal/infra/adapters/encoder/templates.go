package encoder

import "github.com/sa6mwa/mkpod/internal/app/model"

type templateValues struct {
	Atom         *model.Atom
	Episode      *model.Episode
	MetadataFile string
}

var lameCommandTemplate string = `{{ $PRE := "" }}{{ if ne .Atom.LocalStorageDirExpanded "" }}{{ $PRE = print .Atom.LocalStorageDirExpanded "/" }}{{ end }}{{ .Atom.LamepathExpanded }} -b {{ .Atom.Encoding.Bitrate }} {{ escape (print $PRE .Episode.Input) }} {{ escape (print $PRE .Episode.Output) }}`

var ffmpegCommandTemplate string = `{{ $PRE := ""}}{{ if ne .Atom.LocalStorageDirExpanded ""}}{{ $PRE = print .Atom.LocalStorageDirExpanded "/"}}{{ end }}{{ .Atom.FFmpegPathExpanded }} -y -i {{ escape (print $PRE .Episode.Input) }} -pix_fmt yuv420p -colorspace bt709 -color_trc bt709 -color_primaries bt709 -color_range tv -c:v libx264 -profile:v high -crf {{ .Atom.Encoding.CRF }} -maxrate 1M -bufsize 2M -preset medium -coder 1 -movflags +faststart -x264-params open-gop=0 -c:a libfdk_aac -profile:a aac_low -b:a {{ .Atom.Encoding.ABR }} {{ escape (print $PRE .Episode.Output) }}`

// ffmpeg to Lame
var ffmpegToAudioCommandTemplate string = `{{ $PRE := ""}}{{ if ne .Atom.LocalStorageDirExpanded ""}}{{ $PRE = print .Atom.LocalStorageDirExpanded "/"}}{{ end }}{{ .Atom.FFmpegPathExpanded }} -y -i {{ escape (print $PRE .Episode.Input) }} -vn -f wav -c:a pcm_s16le -ac 2 pipe: | {{ .Atom.LamepathExpanded }} -b {{ .Atom.Encoding.Bitrate }} --add-id3v2 --tv TLAN={{ if ne .Episode.EncodingLanguage "" }}{{ escape .Episode.EncodingLanguage }}{{ else }}{{ escape .Atom.Encoding.Language }}{{ end }} --tt {{ escape .Episode.Title }} --ta {{ escape .Atom.Author }} --tl {{ escape .Atom.Title }} --ty {{ escape (.Episode.PubDate.Format "2006") }} --tc {{ escape .Episode.Subtitle }} --tn {{ .Episode.UID }} --tg {{ escape .Atom.Encoding.Genre }} --ti {{ escape (print $PRE .Atom.Encoding.Coverfront) }} --tv WOAR={{ escape .Atom.Link }} - {{ escape (print $PRE .Episode.Output) }}`

// Used to make m4a or m4b audio files. Combines the audio, conver
// image, and metadata with chapters into the output m4a/m4b in a
// single run. FFmpeg has had issues with this approach leading to
// chapter titles missing, corrupt attached pic cover image, and no
// cover image. FFmpeg must be later than november 2023 for chapter
// titles to be included in the output and possibly a 2025 version
// or later for this combined approach to work. A workaround is a
// two-phase run, first writes the audio and metadata, second run
// adds the attached pic image. Build the latest FFmpeg with the
// build script included in the repo if this is an issue. The
// resulting m4a with chapters does however work really well in
// AntennaPod and VLC.
var ffmpegToM4ACommandTemplate string = `{{ $PRE := "" }}{{ if ne .Atom.LocalStorageDirExpanded ""}}{{ $PRE = print .Atom.LocalStorageDirExpanded "/"}}{{ end }}{{ .Atom.FFmpegPathExpanded }} -y -i {{ escape (print $PRE .Episode.Input) }} -i {{ escape (print $PRE .Atom.Encoding.Coverfront) }} -i {{ escape .MetadataFile }} -map 0:a -c:a libfdk_aac -profile:a aac_low -b:a {{ .Atom.Encoding.ABR }} -metadata:s:a:0 language={{ if ne .Episode.EncodingLanguage "" }}{{ escape .Episode.EncodingLanguage }}{{ else }}{{ escape .Atom.Encoding.Language }}{{ end }} -map 1:v -c:v mjpeg -disposition:v:0 attached_pic -metadata:s:v title="Cover" -metadata:s:v comment="Cover (front)" -map_metadata 2 -map_chapters 2 -movflags faststart {{ escape (print $PRE .Episode.Output) }}`
