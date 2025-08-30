package awshandler

import (
	"context"
	"testing"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/app/ports"
)

// mockAsker is a simple mock implementation of ForAsking for testing
type mockAsker struct {
	response bool
}

func (m *mockAsker) Ask(ctx context.Context, format string, a ...any) bool {
	return m.response
}

func TestNew(t *testing.T) {
	atom := &model.Atom{
		Config: model.Config{
			Aws: model.AwsConfig{
				Profile: "test-profile",
				Region:  "us-east-1",
			},
		},
	}
	asker := &mockAsker{response: true}

	handler := New(atom, asker)
	if handler == nil {
		t.Fatal("New() returned nil")
	}

	// Test that it implements the interface
	var _ ports.ForAdministeringRemoteFiles = handler
}

func TestValidStorageClasses(t *testing.T) {
	classes := ports.ValidStorageClasses()
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
		{"standard", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.storageClass, func(t *testing.T) {
			result := ports.IsValidStorageClass(tt.storageClass)
			if result != tt.expected {
				t.Errorf("IsValidStorageClass(%q) = %v, expected %v", tt.storageClass, result, tt.expected)
			}
		})
	}
}

func TestDefaultStorageClass(t *testing.T) {
	if ports.DefaultStorageClass != "INTELLIGENT_TIERING" {
		t.Errorf("DefaultStorageClass = %q, expected %q", ports.DefaultStorageClass, "INTELLIGENT_TIERING")
	}
}

func TestForAdministeringRemoteFilesRequestValidation(t *testing.T) {
	atom := &model.Atom{
		Config: model.Config{
			Aws: model.AwsConfig{
				Profile: "test-profile",
				Region:  "us-east-1",
			},
		},
	}
	asker := &mockAsker{response: true}
	handler := New(atom, asker).(*forAdministeringRemoteFiles)
	ctx := context.Background()

	tests := []struct {
		name    string
		request *ports.ForAdministeringRemoteFilesRequest
		wantErr error
	}{
		{
			name:    "nil request",
			request: nil,
			wantErr: ErrNilPointerRequest,
		},
		{
			name: "empty store",
			request: &ports.ForAdministeringRemoteFilesRequest{
				Store: "",
				Key:   "test-key",
			},
			wantErr: ErrEmptyStore,
		},
		{
			name: "empty key",
			request: &ports.ForAdministeringRemoteFilesRequest{
				Store: "test-bucket",
				Key:   "",
			},
			wantErr: ErrEmptyKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test FileExists method for validation
			_, err := handler.FileExists(ctx, tt.request)
			if err != tt.wantErr {
				t.Errorf("FileExists() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
