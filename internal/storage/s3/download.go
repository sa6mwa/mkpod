package s3store

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/sa6mwa/mkpod/internal/app/humanreadable"
	"github.com/sa6mwa/mkpod/internal/logging"
)

func (c *Client) DownloadFile(ctx context.Context, bucket, key string) error {
	l := logger.FromContext(ctx)

	s3path := "s3://" + path.Join(bucket, key)
	localPath := path.Join(c.atom.LocalStorageDirExpanded(), key)
	l.Info("Downloading "+s3path, "bucket", bucket, "key", key, "location", s3path, "output", localPath)

	downloader := s3manager.NewDownloader(c.session)
	dirPath := path.Dir(localPath)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return err
	}

	uploadClient := New(c.atom, c.prompter)
	var keepFile bool
	var file *os.File

	fi, err := os.Stat(localPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			file, err = os.Create(localPath)
			if err != nil {
				return err
			}
			defer file.Close()
		} else {
			return err
		}
	} else {
		size, err := c.GetObjectSize(bucket, key)
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				switch awsErr.Code() {
				case "NotFound", "NoSuchKey":
					l.Info("Remote does not exist, will use local file only", "remote", s3path, "local", localPath)
					if c.prompter != nil && c.prompter.Ask(ctx, "Upload %s to %s?", localPath, s3path) {
						if err := uploadClient.UploadFile(ctx, bucket, key, localPath, &UploadOptions{StorageClass: c.atom.Config.Aws.Buckets.GetStorageClass(bucket)}); err != nil {
							return err
						}
					}
					return nil
				default:
					return err
				}
			}
			return err
		}
		if size == fi.Size() {
			l.Info("Will not download remote as local file size and content-length of remote match", "contentLength", size)
			return nil
		}

		file, err = os.Create(localPath)
		if err != nil {
			return err
		}
		defer func() {
			defer file.Close()
			if !keepFile {
				_ = os.Remove(file.Name())
			}
		}()
	}

	n, err := downloader.Download(file, &awss3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}

	keepFile = true
	l.Info("Download succeeded", "from", s3path, "to", localPath, "size", n, "humanSize", humanreadable.IEC(n))
	return nil
}

func (c *Client) GetObjectSize(bucket, key string) (int64, error) {
	result, err := c.s3.HeadObject(&awss3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, err
	}
	return aws.Int64Value(result.ContentLength), nil
}
