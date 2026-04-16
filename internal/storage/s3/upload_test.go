package s3store

import (
	"context"
	"testing"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

func TestUploadRequestValidation(t *testing.T) {
	client := New(&model.Atom{
		Config: model.Config{
			Aws: model.AwsConfig{
				Profile: "test-profile",
				Region:  "us-east-1",
			},
		},
	}, &mockAsker{response: true})

	ctx := context.Background()
	tests := []struct {
		name    string
		request *UploadRequest
		wantErr error
	}{
		{name: "nil request", request: nil, wantErr: ErrNilPointerRequest},
		{name: "empty store", request: &UploadRequest{Filename: "podcast.rss"}, wantErr: ErrEmptyStore},
		{name: "empty filename", request: &UploadRequest{Store: "test-bucket"}, wantErr: ErrEmptyFilename},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.UploadFile(ctx, tt.request)
			if err != tt.wantErr {
				t.Fatalf("UploadFile() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
