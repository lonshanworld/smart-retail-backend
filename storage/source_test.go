package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveImageSource(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		kind     string
		resolved string
	}{
		{"direct", "https://cdn.example.com/image.jpg", "URL", "https://cdn.example.com/image.jpg"},
		{"drive file", "https://drive.google.com/file/d/abc123/view?usp=sharing", "GOOGLE_PUBLIC", "https://drive.google.com/uc?export=download&id=abc123"},
		{"drive query", "https://drive.google.com/open?id=abc123", "GOOGLE_PUBLIC", "https://drive.google.com/uc?export=download&id=abc123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveImageSource(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind != tt.kind || got.ResolvedURL != tt.resolved {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestResolveImageSourceRejectsInvalidURL(t *testing.T) {
	if _, err := ResolveImageSource("javascript:alert(1)"); err == nil {
		t.Fatal("expected invalid URL error")
	}
}

func TestValidateRemoteImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("png bytes"))
	}))
	defer server.Close()
	source, err := ResolveImageSource(server.URL + "/image.png")
	if err != nil {
		t.Fatal(err)
	}
	validated, err := ValidateRemoteImage(context.Background(), source, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if validated.ContentType != "image/png" {
		t.Fatalf("unexpected content type: %s", validated.ContentType)
	}
}
