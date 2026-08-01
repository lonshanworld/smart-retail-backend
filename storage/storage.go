package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	cloudinary "github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Object struct {
	Provider    string
	Bucket      string
	ObjectName  string
	PublicID    string
	PublicURL   string
	ContentType string
	Size        int64
}

type UploadInput struct {
	Reader      io.Reader
	Size        int64
	Filename    string
	ContentType string
	Folder      string
}

type Provider interface {
	Name() string
	Upload(context.Context, UploadInput) (Object, error)
	Delete(context.Context, Object) error
}

type Config struct {
	Provider       string
	MaxUploadBytes int64
	CloudName      string
	CloudAPIKey    string
	CloudAPISecret string
	CloudFolder    string
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioRegion    string
	MinioPublicURL string
	MinioSecure    bool
}

func LoadConfig() Config {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("STORAGE_PROVIDER")))
	if provider == "" {
		provider = "minio"
	}
	maxBytes := int64(10 * 1024 * 1024)
	if raw := strings.TrimSpace(os.Getenv("STORAGE_MAX_UPLOAD_BYTES")); raw != "" {
		var parsed int64
		if _, err := fmt.Sscan(raw, &parsed); err == nil && parsed > 0 {
			maxBytes = parsed
		}
	}
	return Config{
		Provider: provider, MaxUploadBytes: maxBytes,
		CloudName: os.Getenv("CLOUDINARY_CLOUD_NAME"), CloudAPIKey: os.Getenv("CLOUDINARY_API_KEY"), CloudAPISecret: os.Getenv("CLOUDINARY_API_SECRET"), CloudFolder: os.Getenv("CLOUDINARY_FOLDER"),
		MinioEndpoint: os.Getenv("MINIO_ENDPOINT"), MinioAccessKey: os.Getenv("MINIO_ACCESS_KEY"), MinioSecretKey: os.Getenv("MINIO_SECRET_KEY"), MinioBucket: os.Getenv("MINIO_BUCKET"), MinioRegion: os.Getenv("MINIO_REGION"), MinioPublicURL: os.Getenv("MINIO_PUBLIC_URL"), MinioSecure: strings.EqualFold(os.Getenv("MINIO_SECURE"), "true"),
	}
}

func NewFromEnv() (Provider, error) {
	cfg := LoadConfig()
	switch cfg.Provider {
	case "cloudinary":
		return NewCloudinary(cfg)
	case "minio":
		return NewMinio(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage provider %q", cfg.Provider)
	}
}

type cloudinaryProvider struct {
	client *cloudinary.Cloudinary
	folder string
}

func NewCloudinary(cfg Config) (Provider, error) {
	if cfg.CloudName == "" || cfg.CloudAPIKey == "" || cfg.CloudAPISecret == "" {
		return nil, fmt.Errorf("cloudinary configuration is incomplete")
	}
	client, err := cloudinary.NewFromParams(cfg.CloudName, cfg.CloudAPIKey, cfg.CloudAPISecret)
	if err != nil {
		return nil, err
	}
	return &cloudinaryProvider{client: client, folder: cfg.CloudFolder}, nil
}

func (p *cloudinaryProvider) Name() string { return "CLOUDINARY" }

func (p *cloudinaryProvider) Upload(ctx context.Context, input UploadInput) (Object, error) {
	params := uploader.UploadParams{Folder: p.folder, ResourceType: "image"}
	result, err := p.client.Upload.Upload(ctx, input.Reader, params)
	if err != nil {
		return Object{}, err
	}
	return Object{Provider: p.Name(), PublicID: result.PublicID, PublicURL: result.SecureURL, ContentType: input.ContentType, Size: input.Size}, nil
}

func (p *cloudinaryProvider) Delete(ctx context.Context, object Object) error {
	if object.PublicID == "" {
		return nil
	}
	_, err := p.client.Upload.Destroy(ctx, uploader.DestroyParams{PublicID: object.PublicID, ResourceType: "image"})
	return err
}

type minioProvider struct {
	client *minio.Client
	cfg    Config
}

func NewMinio(cfg Config) (Provider, error) {
	if cfg.MinioEndpoint == "" || cfg.MinioAccessKey == "" || cfg.MinioSecretKey == "" || cfg.MinioBucket == "" {
		return nil, fmt.Errorf("minio configuration is incomplete")
	}
	client, err := minio.New(cfg.MinioEndpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""), Secure: cfg.MinioSecure, Region: cfg.MinioRegion})
	if err != nil {
		return nil, err
	}
	return &minioProvider{client: client, cfg: cfg}, nil
}

func (p *minioProvider) Name() string { return "MINIO" }

func (p *minioProvider) Upload(ctx context.Context, input UploadInput) (Object, error) {
	objectName := filepath.ToSlash(strings.Trim(strings.TrimSpace(input.Folder), "/") + "/" + safeFilename(input.Filename))
	if strings.HasPrefix(objectName, "/") {
		objectName = strings.TrimPrefix(objectName, "/")
	}
	_, err := p.client.PutObject(ctx, p.cfg.MinioBucket, objectName, input.Reader, input.Size, minio.PutObjectOptions{ContentType: input.ContentType, ContentDisposition: "inline"})
	if err != nil {
		return Object{}, err
	}
	publicURL := p.cfg.MinioPublicURL
	if publicURL == "" {
		scheme := "http"
		if p.cfg.MinioSecure {
			scheme = "https"
		}
		publicURL = scheme + "://" + p.cfg.MinioEndpoint
	}
	return Object{Provider: p.Name(), Bucket: p.cfg.MinioBucket, ObjectName: objectName, PublicURL: strings.TrimRight(publicURL, "/") + "/" + p.cfg.MinioBucket + "/" + objectName, ContentType: input.ContentType, Size: input.Size}, nil
}

func (p *minioProvider) Delete(ctx context.Context, object Object) error {
	if object.ObjectName == "" {
		return nil
	}
	return p.client.RemoveObject(ctx, p.cfg.MinioBucket, object.ObjectName, minio.RemoveObjectOptions{})
}

func safeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "..", "_")
	if name == "." || name == "" {
		return "upload.bin"
	}
	return name
}
