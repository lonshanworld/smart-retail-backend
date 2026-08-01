package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type ImageSource struct {
	Kind        string
	OriginalURL string
	ResolvedURL string
}

type ImageValidation struct {
	ContentType string
	Size        int64
	Width       int
	Height      int
}

var driveFilePattern = regexp.MustCompile(`/file/d/([^/]+)`)

func ResolveImageSource(raw string) (ImageSource, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ImageSource{}, fmt.Errorf("image URL must be a valid HTTP or HTTPS URL")
	}
	if strings.Contains(strings.ToLower(u.Host), "drive.google.com") || strings.Contains(strings.ToLower(u.Host), "docs.google.com") {
		fileID := ""
		if match := driveFilePattern.FindStringSubmatch(u.Path); len(match) == 2 {
			fileID = match[1]
		}
		if fileID == "" {
			fileID = u.Query().Get("id")
		}
		if fileID == "" {
			return ImageSource{}, fmt.Errorf("could not extract a public Google Drive file ID")
		}
		return ImageSource{Kind: "GOOGLE_PUBLIC", OriginalURL: raw, ResolvedURL: "https://drive.google.com/uc?export=download&id=" + url.QueryEscape(fileID)}, nil
	}
	return ImageSource{Kind: "URL", OriginalURL: raw, ResolvedURL: raw}, nil
}

func ValidateRemoteImage(ctx context.Context, source ImageSource, maxBytes int64) (ImageValidation, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.ResolvedURL, nil)
	if err != nil {
		return ImageValidation{}, fmt.Errorf("invalid image URL: %w", err)
	}
	request.Header.Set("Range", "bytes=0-1048575")
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return ImageValidation{}, fmt.Errorf("image URL is not reachable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ImageValidation{}, fmt.Errorf("image URL returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxBytes && response.ContentLength >= 0 {
		return ImageValidation{}, fmt.Errorf("remote image exceeds the configured size limit")
	}
	header, err := io.ReadAll(io.LimitReader(response.Body, 512))
	if err != nil {
		return ImageValidation{}, fmt.Errorf("failed to inspect remote image: %w", err)
	}
	contentType := response.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		contentType = http.DetectContentType(header)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return ImageValidation{}, fmt.Errorf("remote URL does not contain an image")
	}
	return ImageValidation{ContentType: contentType, Size: response.ContentLength}, nil
}
