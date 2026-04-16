package s3store

import (
	"context"
	"testing"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

type mockAsker struct {
	response bool
}

func (m *mockAsker) Ask(ctx context.Context, format string, a ...any) bool {
	return m.response
}

func TestNewAdminClient(t *testing.T) {
	atom := &model.Podcast{
		Config: model.Config{
			Aws: model.AwsConfig{
				Profile: "test-profile",
				Region:  "us-east-1",
			},
		},
	}
	asker := &mockAsker{response: true}

	handler := NewAdminClient(atom, asker)
	if handler == nil {
		t.Fatal("NewAdminClient() returned nil")
	}
}

func TestValidStorageClasses(t *testing.T) {
	classes := ValidStorageClasses()
	expectedClasses := []string{
		"STANDARD",
		"REDUCED_REDUNDANCY",
		"STANDARD_IA",
		"ONEZONE_IA",
		"INTELLIGENT_TIERING",
		"GLACIER",
		"DEEP_ARCHIVE",
		"GLACIER_IR",
	}

	if len(classes) != len(expectedClasses) {
		t.Errorf("Expected %d storage classes, got %d", len(expectedClasses), len(classes))
	}

	for i, expected := range expectedClasses {
		if i >= len(classes) || classes[i] != expected {
			t.Errorf("Expected storage class %q at index %d, got %q", expected, i, classes[i])
		}
	}
}

func TestIsValidStorageClass(t *testing.T) {
	tests := []struct {
		storageClass string
		expected     bool
	}{
		{"STANDARD", true},
		{"INTELLIGENT_TIERING", true},
		{"GLACIER", true},
		{"INVALID_CLASS", false},
		{"", false},
		{"standard", false},
	}

	for _, tt := range tests {
		t.Run(tt.storageClass, func(t *testing.T) {
			result := IsValidStorageClass(tt.storageClass)
			if result != tt.expected {
				t.Errorf("IsValidStorageClass(%q) = %v, expected %v", tt.storageClass, result, tt.expected)
			}
		})
	}
}

func TestDefaultStorageClass(t *testing.T) {
	if DefaultStorageClass != "INTELLIGENT_TIERING" {
		t.Errorf("DefaultStorageClass = %q, expected %q", DefaultStorageClass, "INTELLIGENT_TIERING")
	}
}

func TestAdminRequestValidation(t *testing.T) {
	atom := &model.Podcast{
		Config: model.Config{
			Aws: model.AwsConfig{
				Profile: "test-profile",
				Region:  "us-east-1",
			},
		},
	}
	asker := &mockAsker{response: true}
	handler := NewAdminClient(atom, asker)
	ctx := context.Background()

	tests := []struct {
		name    string
		request *ObjectRequest
		wantErr error
	}{
		{name: "nil request", request: nil, wantErr: ErrNilPointerRequest},
		{name: "empty store", request: &ObjectRequest{Store: "", Key: "test-key"}, wantErr: ErrEmptyStore},
		{name: "empty key", request: &ObjectRequest{Store: "test-bucket", Key: ""}, wantErr: ErrEmptyKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler.FileExists(ctx, tt.request)
			if err != tt.wantErr {
				t.Errorf("FileExists() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
