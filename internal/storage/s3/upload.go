package s3store

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
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
	if err := c.ensureClient(ctx); err != nil {
		return err
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

	storageClassValue := s3types.StorageClass(storageClass)
	uploader := c.newUploader()
	result, err := uploader.Upload(ctx, &awss3.PutObjectInput{
		Bucket:       &bucket,
		Key:          &key,
		ContentType:  &contentType,
		Body:         file,
		StorageClass: storageClassValue,
	})
	if err != nil {
		return err
	}
	l.Info("Upload succeeded", "location", result.Location)
	return nil
}

// DiffTextObject downloads the current S3 object and prints a unified diff
// against the given local file.
func (c *Client) DiffTextObject(ctx context.Context, bucket, key, fileToDiff string) error {
	fileContent, err := os.ReadFile(fileToDiff)
	if err != nil {
		return err
	}
	return c.diffTextObjectContent(ctx, bucket, key, fileToDiff, fileContent)
}

// DiffTextObjectBytes downloads the current S3 object and prints a unified diff
// against the given in-memory content.
func (c *Client) DiffTextObjectBytes(ctx context.Context, bucket, key, label string, content []byte) error {
	if strings.TrimSpace(label) == "" {
		label = "<generated>"
	}
	return c.diffTextObjectContent(ctx, bucket, key, label, content)
}

func (c *Client) diffTextObjectContent(ctx context.Context, bucket, key, label string, content []byte) error {
	l := logger.FromContext(ctx)
	if err := c.ensureClient(ctx); err != nil {
		return err
	}

	downloader := c.newDownloader()
	buf := manager.NewWriteAtBuffer([]byte{})
	size, err := downloader.Download(ctx, buf, &awss3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		if isNotFoundError(err) {
			l.Info("Skipping diff", "file", label, "path", "s3://"+path.Join(bucket, key), "error", err)
			return nil
		}
		return err
	}

	l.Info("Buffered successfully", "path", "s3://"+path.Join(bucket, key), "bytes", size)
	l.Info("Diff follows", "to", label, "from", "s3://"+path.Join(bucket, key))

	edits := myers.ComputeEdits(span.URIFromPath("s3://"+path.Join(bucket, key)), string(buf.Bytes()), string(content))
	diff := fmt.Sprint(gotextdiff.ToUnified("s3://"+path.Join(bucket, key), label, string(buf.Bytes()), edits))
	fmt.Println(diff)
	return nil
}
