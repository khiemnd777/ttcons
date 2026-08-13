package storage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

const (
	S3Provider                     = "s3"
	defaultStorageOperationTimeout = 30 * time.Second
)

type S3Config struct {
	Bucket           string
	Region           string
	Endpoint         string
	UsePathStyle     bool
	OperationTimeout time.Duration
	HTTPClient       aws.HTTPClient
}

type s3Client interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type S3 struct {
	bucket           string
	operationTimeout time.Duration
	client           s3Client
}

func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	if cfg.Bucket == "" {
		return nil, errors.New("S3 storage bucket is empty")
	}
	if cfg.Region == "" {
		return nil, errors.New("S3 storage region is empty")
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = defaultStorageOperationTimeout
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithRetryer(func() aws.Retryer {
			return retry.NewStandard(func(options *retry.StandardOptions) {
				options.MaxAttempts = 3
				options.MaxBackoff = 2 * time.Second
			})
		}),
		awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	}
	if cfg.HTTPClient != nil {
		loadOptions = append(loadOptions, awsconfig.WithHTTPClient(cfg.HTTPClient))
	}
	loaded, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}
	client := s3.NewFromConfig(loaded, func(options *s3.Options) {
		options.UsePathStyle = cfg.UsePathStyle
		if cfg.Endpoint != "" {
			options.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})
	return newS3WithClient(cfg, client)
}

func newS3WithClient(cfg S3Config, client s3Client) (*S3, error) {
	if client == nil {
		return nil, errors.New("S3 client is nil")
	}
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, errors.New("S3 storage bucket is empty")
	}
	timeout := cfg.OperationTimeout
	if timeout <= 0 {
		timeout = defaultStorageOperationTimeout
	}
	return &S3{bucket: bucket, operationTimeout: timeout, client: client}, nil
}

func (s *S3) Provider() string { return S3Provider }

func (s *S3) Put(ctx context.Context, key string, source io.Reader, options PutOptions) (ObjectInfo, error) {
	if source == nil {
		return ObjectInfo{}, errors.New("storage source is nil")
	}
	if _, err := storageKeySegments(key); err != nil {
		return ObjectInfo{}, err
	}
	if options.ExpectedSize < 0 {
		return ObjectInfo{}, errors.New("S3 storage requires a known expected size")
	}
	seekable, ok := source.(io.ReadSeeker)
	if !ok {
		return ObjectInfo{}, errors.New("S3 storage source must be seekable")
	}
	operationContext, cancel := context.WithTimeout(ctx, s.operationTimeout)
	defer cancel()

	start, err := seekable.Seek(0, io.SeekCurrent)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("resolve S3 upload source position: %w", err)
	}
	hasher := sha256.New()
	hashSource := io.Reader(&contextReader{ctx: operationContext, reader: seekable})
	const maxInt64 = int64(^uint64(0) >> 1)
	if options.ExpectedSize < maxInt64 {
		hashSource = io.LimitReader(hashSource, options.ExpectedSize+1)
	}
	written, err := io.Copy(hasher, hashSource)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("hash S3 upload source: %w", err)
	}
	if written != options.ExpectedSize {
		return ObjectInfo{}, fmt.Errorf("storage size mismatch: expected %d, read %d", options.ExpectedSize, written)
	}
	if _, err := seekable.Seek(start, io.SeekStart); err != nil {
		return ObjectInfo{}, fmt.Errorf("rewind S3 upload source: %w", err)
	}
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          seekable,
		ContentType:   aws.String(normalizedContentType(options.ContentType)),
		IfNoneMatch:   aws.String("*"),
		ContentLength: aws.Int64(options.ExpectedSize),
	}
	_, err = s.client.PutObject(operationContext, input)
	if err != nil {
		if apiErrorCode(err, "PreconditionFailed", "ConditionalRequestConflict") {
			return ObjectInfo{}, fmt.Errorf("%w: %s", ErrObjectExists, key)
		}
		return ObjectInfo{}, fmt.Errorf("put S3 object: %w", err)
	}
	return ObjectInfo{
		Provider:       S3Provider,
		Key:            key,
		Size:           written,
		ContentType:    normalizedContentType(options.ContentType),
		ChecksumSHA256: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func (s *S3) Open(ctx context.Context, key string) (Object, error) {
	if _, err := storageKeySegments(key); err != nil {
		return Object{}, err
	}
	operationContext, cancel := context.WithTimeout(ctx, s.operationTimeout)
	output, err := s.client.GetObject(operationContext, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		cancel()
		if apiErrorCode(err, "NoSuchKey", "NotFound", "NoSuchObject") {
			return Object{}, fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return Object{}, fmt.Errorf("open S3 object: %w", err)
	}
	if output.Body == nil {
		cancel()
		return Object{}, errors.New("open S3 object returned an empty body")
	}
	return Object{
		Reader: &cancelReadCloser{ReadCloser: output.Body, cancel: cancel},
		Info: ObjectInfo{
			Provider:       S3Provider,
			Key:            key,
			Size:           aws.ToInt64(output.ContentLength),
			ContentType:    normalizedContentType(aws.ToString(output.ContentType)),
			ChecksumSHA256: checksumBase64ToHex(aws.ToString(output.ChecksumSHA256)),
		},
	}, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if _, err := storageKeySegments(key); err != nil {
		return err
	}
	operationContext, cancel := context.WithTimeout(ctx, s.operationTimeout)
	defer cancel()
	_, err := s.client.DeleteObject(operationContext, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete S3 object: %w", err)
	}
	return nil
}

func normalizedContentType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "application/octet-stream"
	}
	return value
}

func checksumBase64ToHex(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return ""
	}
	return hex.EncodeToString(decoded)
}

func apiErrorCode(err error, codes ...string) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	for _, code := range codes {
		if apiErr.ErrorCode() == code {
			return true
		}
	}
	return false
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	count, err := r.reader.Read(buffer)
	r.count += int64(count)
	return count, err
}

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r *cancelReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.cancel()
	return err
}
