package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// s3API is the narrow S3 surface used by the upload lib. The production
// client is *s3.Client; tests inject a fake.
type s3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type s3Presigner interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4PresignedURL, error)
}

// v4PresignedURL is a minimal stand-in for the SDK's presigned result.
type v4PresignedURL struct {
	URL string
}

type s3Backend struct {
	bucket    string
	prefix    string
	client    s3API
	presigner s3Presigner
}

func newS3Backend(cfg S3Config) (*s3Backend, error) {
	client, presigner, err := buildS3Client(cfg)
	if err != nil {
		return nil, err
	}
	return &s3Backend{
		bucket:    cfg.Bucket,
		prefix:    cfg.normalizedPrefix(),
		client:    client,
		presigner: presigner,
	}, nil
}

// newS3BackendWithClient is test seam — caller supplies a fake s3API.
func newS3BackendWithClient(cfg S3Config, client s3API, presigner s3Presigner) *s3Backend {
	return &s3Backend{
		bucket:    cfg.Bucket,
		prefix:    cfg.normalizedPrefix(),
		client:    client,
		presigner: presigner,
	}
}

func buildS3Client(cfg S3Config) (s3API, s3Presigner, error) {
	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	opts := func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	}

	client := s3.NewFromConfig(awsCfg, opts)
	presignClient := s3.NewPresignClient(client)

	// Wrap presigner to satisfy s3Presigner interface.
	wrapped := &sdkPresigner{client: presignClient}

	return client, wrapped, nil
}

type sdkPresigner struct {
	client *s3.PresignClient
}

func (p *sdkPresigner) PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4PresignedURL, error) {
	out, err := p.client.PresignGetObject(ctx, params, optFns...)
	if err != nil {
		return nil, err
	}
	return &v4PresignedURL{URL: out.URL}, nil
}

func (b *s3Backend) put(ctx context.Context, key, mimeType string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return wrapError(ErrTimeout, "s3", "upload", err)
	}
	fullKey := b.prefix + key
	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(fullKey),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(mimeType),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline") {
			return wrapError(ErrTimeout, "s3", "upload", err)
		}
		return wrapError(ErrTransport, "s3", "upload", fmt.Errorf("PutObject %s: %w", fullKey, err))
	}
	return nil
}

func (b *s3Backend) delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return wrapError(ErrTimeout, "s3", "delete", err)
	}
	fullKey := b.prefix + key
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		// S3 DeleteObject is idempotent — treat NotFound as success, but
		// surface transport errors.
		if isS3NotFound(err) {
			return nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return wrapError(ErrTimeout, "s3", "delete", err)
		}
		return wrapError(ErrTransport, "s3", "delete", err)
	}
	return nil
}

func (b *s3Backend) presignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if b.presigner == nil {
		return "", wrapError(ErrBadRequest, "s3", "presign", fmt.Errorf("presigner not configured"))
	}
	if expiry < time.Minute || expiry > 7*24*time.Hour {
		return "", wrapError(ErrBadRequest, "s3", "presign", fmt.Errorf("expiry must be between 1m and 7d, got %s", expiry))
	}
	if err := ctx.Err(); err != nil {
		return "", wrapError(ErrTimeout, "s3", "presign", err)
	}
	fullKey := b.prefix + key
	out, err := b.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(fullKey),
	}, func(o *s3.PresignOptions) {
		o.Expires = expiry
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", wrapError(ErrTimeout, "s3", "presign", err)
		}
		return "", wrapError(ErrTransport, "s3", "presign", err)
	}
	return out.URL, nil
}

func isS3NotFound(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "NotFound") || strings.Contains(s, "NoSuchKey") || strings.Contains(s, "404")
}
