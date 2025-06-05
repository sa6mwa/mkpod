// downloader is the default AWS v1 download handler, indepentent of
// the default AWS upload adapter.
package downloader

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/sa6mwa/mkpod/internal/app/humanreadable"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/app/ports"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/uploader"
)

type forDownloading struct {
	ports.ForAsking
	atom    *model.Atom
	session *session.Session
	s3      *s3.S3
}

func New(askerAdapter ports.ForAsking, atom *model.Atom) ports.ForDownloading {
	s := session.Must(session.NewSessionWithOptions(session.Options{
		Profile: atom.Config.Aws.Profile,
		Config: aws.Config{
			Region: aws.String(atom.Config.Aws.Region),
		},
	}))
	return &forDownloading{
		ForAsking: askerAdapter,
		atom:      atom,
		session:   s,
		s3:        s3.New(s),
	}
}

func (d *forDownloading) Download(ctx context.Context, bucket, key string) error {
	l := logger.FromContext(ctx)

	// Create a new AWS uploader (not a pluggable port, tightly coupled to this AWS implementation)
	ul := uploader.New(d.atom)

	s3path := "s3://" + path.Join(bucket, key)
	completePath := path.Join(d.atom.LocalStorageDirExpanded(), key)
	l.Info("Downloading "+s3path, "bucket", bucket, "key", key, "location", s3path, "output", completePath)

	downloader := s3manager.NewDownloader(d.session)
	dirPath := path.Dir(completePath)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}
	var keepFile bool = false // only used if os.Create is involved
	var f *os.File
	fi, err := os.Stat(completePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			f, err = os.Create(completePath)
			if err != nil {
				return err
			}
			defer f.Close()
		} else {
			return err
		}
	} else {
		// No error, could stat file (file exists).
		// Get content length of file in S3 bucket and compare size to
		// file already on disk, do not download if they match.
		size, err := d.GetSize(ctx, bucket, key)
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				switch awsErr.Code() {
				case "NotFound", "NoSuchKey":
					l.Info("Remote does not exist, will use local file only", "remote", s3path, "local", completePath)
					// Upload local file to bucket with key?
					if d.Ask(ctx, "Upload %s to %s?", completePath, s3path) {
						// Upload
						sc := d.atom.Config.Aws.Buckets.GetStorageClass(bucket)
						if err := ul.Upload(ctx, &ports.ForUploadingRequest{
							Store:        bucket,
							To:           key,
							From:         completePath,
							StorageClass: sc,
						}); err != nil {
							return err
						}
					}
					return nil
				default:
					return err
				}
			} else {
				return err
			}
		}
		// OK, got size of already present file
		if size != fi.Size() {
			// Size does not match, download file (truncate file and fall through)
			f, err := os.Create(completePath)
			if err != nil {
				return err
			}
			defer func() {
				defer f.Close()
				if !keepFile {
					os.Remove(f.Name())
				}
			}()
			// Continues outside...
		} else {
			// Size match
			l.Info("Will not download remote as local file size and content-length of remote match", "contentLength", size)
			return nil
		}
	}
	n, err := downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	// No errors, keep file
	keepFile = true
	l.Info("Download succeeded", "from", s3path, "to", completePath, "size", n, "humanSize", humanreadable.IEC(n))
	return nil
}

func (d *forDownloading) GetSize(_ context.Context, bucket, key string) (int64, error) {
	result, err := d.s3.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, err
	}
	return aws.Int64Value(result.ContentLength), nil
}
