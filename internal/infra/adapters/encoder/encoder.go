// encoder is the default file-based encoder of master audio or video
// files into published mp3, m4a or mp4 outputs. Implements the
// ports.ForEncoding interface.
package encoder

import (
	"context"
	"path"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/app/ports"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
)

type forEncoding struct {
	ports.ForAsking
	atom *model.Atom
}

func New(askerAdapter ports.ForAsking, atom *model.Atom) ports.ForEncoding {

	return &forEncoding{
		ForAsking: askerAdapter,
		atom:      atom,
	}
}

func (e *forEncoding) Encode(ctx context.Context, atom *model.Atom, uid int64) error {
	var indexes []int = make([]int, 0)

	l := logger.FromContext(ctx)
	mimetype.SetLimit(1024 * 1024)

	if uid < 0 {
		// Iterate all episodes into the indexes slice
		for i, _ := range e.atom.Episodes {
			indexes = append(indexes, i)
		}
	} else {
		if idx := e.atom.ContainsEpisode(uid); idx >= 0 {
			indexes = append(indexes, int(idx))
		} else {
			l.Warn("Episode does not exist in pod specification, skipping", "uid", uid)
			return nil
		}
	}

	// Iterate over one episode or all episodes depending on value of
	// uid.
	for _, i := range indexes {
		inputPath := path.Join(e.atom.LocalStorageDirExpanded(), e.atom.Episodes[i].Input)
		inputContentType, err := GetFileContentType(inputPath)
		if err != nil {
			return err
		}
		format := strings.TrimSpace(strings.ToLower(e.atom.Episodes[i].Format))

		// If input content type is video/* and format is not "audio",
		// we are to encode it using ffmpeg to an mp4. If format is
		// "audio", drop the video stream and encode an mp3 (audio
		// only).
		if strings.HasPrefix(inputContentType, "video/") {
			// If episode format is video or mp4, it's a video episode.
			switch format {
			case "", "video", "mp4":
				//continue here, implement EncodeMP4 function
				if err := EncodeMP4
			}

		}

		//continue here

	}

}

func GetFileContentType(filename string) (contentType string, err error) {
	mimetype.SetLimit(1024 * 1024)
	mimeType, err := mimetype.DetectFile(filename)
	if err != nil {
		return "", err
	}
	return mimeType.String(), nil
}
