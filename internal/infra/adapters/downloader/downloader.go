// downloader is the default AWS v1 download handler, indepentent of
// the default AWS upload adapter.
package downloader

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/app/ports"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
)

type forDownloading struct {
	atom    *model.Atom
	session *session.Session
	s3      *s3.S3
}

func New(atom *model.Atom) ports.ForDownloading {
	s := session.Must(session.NewSessionWithOptions(session.Options{
		Profile: atom.Config.Aws.Profile,
		Config: aws.Config{
			Region: aws.String(atom.Config.Aws.Region),
		},
	}))
	return &forDownloading{
		atom:    atom,
		session: s,
		s3:      s3.New(s),
	}
}

func (d *forDownloading) DownloadFile(ctx context.Context, bucket, key string) error {
	l := logger.FromContext(ctx)
	return nil
}

func (d *forDownloading) GetSize(ctx context.Context, bucket, key string) (int64, error) {
	l := logger.FromContext(ctx)
	return 0, nil
}
