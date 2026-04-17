package s3store

import (
	"context"
	"testing"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

func TestUploadRequestValidation(t *testing.T) {
	client := New(&model.Podcast{
		Config: model.Config{
			Aws: model.AwsConfig{
				Profile: "test-profile",
				Region:  "us-east-1",
			},
		},
	}, &mockAsker{response: true})

	ctx := context.Background()
	tests := []struct {
		name     string
		bucket   string
		key      string
		filename string
		options  *UploadOptions
		wantErr  error
	}{
		{name: "empty store", bucket: "", key: "podcast.rss", filename: "podcast.rss", wantErr: ErrEmptyStore},
		{name: "empty key", bucket: "test-bucket", key: "", filename: "podcast.rss", wantErr: ErrEmptyKey},
		{name: "empty filename", bucket: "test-bucket", key: "podcast.rss", filename: "", wantErr: ErrEmptyFilename},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.UploadFile(ctx, tt.bucket, tt.key, tt.filename, tt.options)
			if err != tt.wantErr {
				t.Fatalf("UploadFile() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
