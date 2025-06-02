// uploader is the default AWS v1 upload handler, independent of the
// default AWS download adapter.
package uploader

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/app/ports"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
)

type forUploading struct {
	atom    *model.Atom
	session *session.Session
	s3      *s3.S3
}

func New(atom *model.Atom) ports.ForUploading {
	s := session.Must(session.NewSessionWithOptions(session.Options{
		Profile: atom.Config.Aws.Profile,
		Config: aws.Config{
			Region: aws.String(atom.Config.Aws.Region),
		},
	}))
	return &forUploading{
		atom:    atom,
		session: s,
		s3:      s3.New(s),
	}
}

func (u *forUploading) UploadFile(ctx context.Context, bucket, contentType, fileToUpload string) error {
	l := logger.FromContext(ctx)
	return nil
}

func (u *forUploading) GetSize(ctx context.Context, bucket, key string) (int64, error) {
	l := logger.FromContext(ctx)
	return 0, nil
}

func (u *forUploading) Diff(ctx context.Context, bucket, key, fileToDiff string) error {
	l := logger.FromContext(ctx)
	return nil
}
