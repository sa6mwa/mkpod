package s3store

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gabriel-vasile/mimetype"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/sa6mwa/mkpod/internal/app/humanreadable"
	logger "github.com/sa6mwa/mkpod/internal/logging"
)

type UploadOptions struct {
	ContentType  string
	StorageClass string
}

func (c *Client) getContentType(filename string) (string, error) {
	mimetype.SetLimit(1024 * 1024)
	mimeType, err := mimetype.DetectFile(filename)
	if err != nil {
		return "", err
	}
	return mimeType.String(), nil
}

func (c *Client) UploadFile(ctx context.Context, bucket, key, filename string, options *UploadOptions) error {
	l := logger.FromContext(ctx)
	if err := validateObjectArgs(bucket, key); err != nil {
		return err
	}
	if strings.TrimSpace(filename) == "" {
		return ErrEmptyFilename
	}
	if options == nil {
		options = &UploadOptions{}
	}

	contentType := strings.TrimSpace(options.ContentType)
	if contentType == "" {
		detectedContentType, err := c.getContentType(filename)
		if err != nil {
			return err
		}
		contentType = detectedContentType
	}

	storageClass := options.StorageClass
	if storageClass == "" {
		storageClass = StorageClassStandard
	}

	s3path := "s3://" + path.Join(bucket, key)
	fi, err := os.Stat(filename)
	if err != nil {
		return err
	}
	l.Info("Uploading to S3", "file", filename, "to", s3path, "storageClass", storageClass, "size", fi.Size(), "humanSize", humanreadable.IEC(fi.Size()))

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	uploader := s3manager.NewUploader(c.session)
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:       aws.String(bucket),
		Key:          aws.String(key),
		ContentType:  aws.String(contentType),
		Body:         file,
		StorageClass: aws.String(storageClass),
	})
	if err != nil {
		return err
	}
	l.Info("Upload succeeded", "location", aws.StringValue(&result.Location))
	return nil
}

// DiffTextObject downloads the current S3 object and prints a unified diff
// against the given local file.
func (c *Client) DiffTextObject(ctx context.Context, bucket, key, fileToDiff string) error {
	l := logger.FromContext(ctx)

	fileContent, err := os.ReadFile(fileToDiff)
	if err != nil {
		return err
	}

	downloader := s3manager.NewDownloader(c.session)
	buf := aws.NewWriteAtBuffer([]byte{})
	size, err := downloader.Download(buf, &awss3.GetObjectInput{
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
		}
		return err
	}

	l.Info("Buffered successfully", "path", "s3://"+path.Join(bucket, key), "bytes", size)
	l.Info("Diff follows", "to", fileToDiff, "from", "s3://"+path.Join(bucket, key))

	edits := myers.ComputeEdits(span.URIFromPath("s3://"+path.Join(bucket, key)), string(buf.Bytes()), string(fileContent))
	diff := fmt.Sprint(gotextdiff.ToUnified("s3://"+path.Join(bucket, key), fileToDiff, string(buf.Bytes()), edits))
	fmt.Println(diff)
	return nil
}
