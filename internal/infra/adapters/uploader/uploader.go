// uploader is the default AWS v1 upload handler, independent of the
// default AWS download adapter.
package uploader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gabriel-vasile/mimetype"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/sa6mwa/mkpod/internal/app/humanreadable"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/app/ports"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
)

var (
	ErrNilPointerRequest error = errors.New("received nil pointer as request")
	ErrFilenameMissing   error = errors.New("empty or missing filename given")
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

func (u *forUploading) getContentType(filename string) (contentType string, err error) {
	mimetype.SetLimit(1024 * 1024)
	mimeType, err := mimetype.DetectFile(filename)
	if err != nil {
		return "", err
	}
	return mimeType.String(), nil
}

// Upload fileToUpload as key to S3 bucket. If ContentType is empty in
// r, function will attempt to detect the content-type of the file in
// the r.From field.
func (u *forUploading) Upload(ctx context.Context, r *ports.ForUploadingRequest) error {
	l := logger.FromContext(ctx)
	if r == nil {
		return ErrNilPointerRequest
	}

	if strings.TrimSpace(r.From) == "" {
		return ErrFilenameMissing
	}

	// Get Content-Type if none was given in the request.
	if strings.TrimSpace(r.ContentType) == "" {
		var err error
		r.ContentType, err = u.getContentType(r.From)
		if err != nil {
			return err
		}
	}

	if strings.TrimSpace(r.To) == "" {
		r.To = r.From
	}
	if r.StorageClass == "" {
		r.StorageClass = "STANDARD"
	}
	s3path := "s3://" + path.Join(r.Store, r.To)
	fi, err := os.Stat(r.From)
	if err != nil {
		return err
	}
	l.Info("Uploading to S3", "file", r.From, "to", s3path, "storageClass", r.StorageClass, "size", fi.Size(), "humanSize", humanreadable.IEC(fi.Size()))
	f, err := os.Open(r.From)
	if err != nil {
		return err
	}
	defer f.Close()
	uploader := s3manager.NewUploader(u.session)
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:       aws.String(r.Store),
		Key:          aws.String(r.To),
		ContentType:  aws.String(r.ContentType),
		Body:         f,
		StorageClass: aws.String(r.StorageClass),
	})
	if err != nil {
		return err
	}
	l.Info("Upload succeeded", "location", aws.StringValue(&result.Location))
	return nil
}

// func (u *forUploading) GetSize(ctx context.Context, bucket, key string) (int64, error) {
// 	l := logger.FromContext(ctx)
// 	s3path := "s3://" + path.Join(bucket, key)
// 	l.Info("Getting size", "location", s3path)
// 	result, err := u.s3.HeadObject(&s3.HeadObjectInput{
// 		Bucket: aws.String(bucket),
// 		Key:    aws.String(key),
// 	})
// 	if err != nil {
// 		return 0, err
// 	}
// 	l.Info("Got size", "size", aws.Int64Value(result.ContentLength), "humanSize", humanreadable.IEC(aws.Int64Value(result.ContentLength)), "location", s3path)
// 	return aws.Int64Value(result.ContentLength), nil
// }

// Diff fileToDiff by downloading from the bucket and compare content
// with the content in fileToDiff. Prints diff to stdout.
func (u *forUploading) Diff(ctx context.Context, bucket, key, fileToDiff string) error {
	l := logger.FromContext(ctx)

	fileContent, err := os.ReadFile(fileToDiff)
	if err != nil {
		return err
	}
	downloader := s3manager.NewDownloader(u.session)
	buf := aws.NewWriteAtBuffer([]byte{})
	size, err := downloader.Download(buf, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "NotFound", "NoSuchKey":
				l.Info("Skipping diff", "file", fileToDiff, "path", "s3://"+path.Join(bucket, key), "error", err)
				return nil
			default:
				return err
			}
		} else {
			return err
		}
	}
	l.Info("Buffered successfully", "path", "s3://"+path.Join(bucket, key), "bytes", size)
	l.Info("Diff follows", "to", fileToDiff, "from", "s3://"+path.Join(bucket, key))

	edits := myers.ComputeEdits(span.URIFromPath("s3://"+path.Join(bucket, key)), string(buf.Bytes()), string(fileContent))
	diff := fmt.Sprint(gotextdiff.ToUnified("s3://"+path.Join(bucket, key), fileToDiff, string(buf.Bytes()), edits))
	fmt.Println(diff)

	return nil
}
