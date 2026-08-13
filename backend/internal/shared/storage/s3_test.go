package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type fakeS3Client struct {
	putInput          *s3.PutObjectInput
	putBody           []byte
	putErr            error
	getInput          *s3.GetObjectInput
	getOutput         *s3.GetObjectOutput
	getErr            error
	deleteInput       *s3.DeleteObjectInput
	deleteErr         error
	waitForPutContext bool
}

func (c *fakeS3Client) PutObject(ctx context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	c.putInput = input
	if c.waitForPutContext {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if input.Body != nil {
		c.putBody, _ = io.ReadAll(input.Body)
	}
	return &s3.PutObjectOutput{}, c.putErr
}

func (c *fakeS3Client) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	c.getInput = input
	return c.getOutput, c.getErr
}

func (c *fakeS3Client) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	c.deleteInput = input
	return &s3.DeleteObjectOutput{}, c.deleteErr
}

func TestS3PutStreamsSizeContentTypeAndChecksum(t *testing.T) {
	client := &fakeS3Client{}
	store, err := newS3WithClient(S3Config{Bucket: "artifacts", OperationTimeout: time.Second}, client)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("protected artifact")
	info, err := store.Put(context.Background(), "evidence/project/object.pdf", bytes.NewReader(payload), PutOptions{
		ContentType:  "application/pdf",
		ExpectedSize: int64(len(payload)),
	})
	if err != nil {
		t.Fatal(err)
	}
	wantChecksum := sha256.Sum256(payload)
	if info.Provider != S3Provider || info.Key != "evidence/project/object.pdf" || info.Size != int64(len(payload)) || info.ChecksumSHA256 != hex.EncodeToString(wantChecksum[:]) {
		t.Fatalf("object info = %#v", info)
	}
	if string(client.putBody) != string(payload) || aws.ToString(client.putInput.IfNoneMatch) != "*" || aws.ToString(client.putInput.ContentType) != "application/pdf" || aws.ToInt64(client.putInput.ContentLength) != int64(len(payload)) {
		t.Fatalf("put input = %#v body=%q", client.putInput, client.putBody)
	}
}

func TestS3PutDoesNotRequireTemporaryFilesystem(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing-temp-directory"))
	client := &fakeS3Client{}
	store, err := newS3WithClient(S3Config{Bucket: "artifacts", OperationTimeout: time.Second}, client)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("memory-only S3 source")
	if _, err := store.Put(context.Background(), "workflow/pdf.pdf", bytes.NewReader(payload), PutOptions{ExpectedSize: int64(len(payload))}); err != nil {
		t.Fatalf("S3 put unexpectedly required TMPDIR: %v", err)
	}
}

func TestS3PutRejectsNonSeekableSourceBeforeCallingClient(t *testing.T) {
	client := &fakeS3Client{}
	store, err := newS3WithClient(S3Config{Bucket: "artifacts", OperationTimeout: time.Second}, client)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Put(context.Background(), "workflow/non-seekable.pdf", bytes.NewBufferString("content"), PutOptions{ExpectedSize: 7})
	if err == nil || !strings.Contains(err.Error(), "seekable") {
		t.Fatalf("non-seekable source error=%v", err)
	}
	if client.putInput != nil {
		t.Fatal("non-seekable source reached S3 client")
	}
}

func TestS3PutRejectsExistingObjectAndSizeMismatch(t *testing.T) {
	client := &fakeS3Client{putErr: &smithy.GenericAPIError{Code: "PreconditionFailed", Message: "exists"}}
	store, _ := newS3WithClient(S3Config{Bucket: "artifacts"}, client)
	_, err := store.Put(context.Background(), "same-key", bytes.NewReader([]byte("abc")), PutOptions{ExpectedSize: 3})
	if !errors.Is(err, ErrObjectExists) {
		t.Fatalf("existing object error=%v", err)
	}

	client.putErr = nil
	client.putInput = nil
	_, err = store.Put(context.Background(), "wrong-size", bytes.NewReader([]byte("abc")), PutOptions{ExpectedSize: 4})
	if err == nil || errors.Is(err, ErrObjectExists) {
		t.Fatalf("size mismatch error=%v", err)
	}
	if client.putInput != nil {
		t.Fatal("size mismatch reached S3 before source validation")
	}
}

func TestS3OpenAllowsMissingChecksumHeader(t *testing.T) {
	client := &fakeS3Client{getOutput: &s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader([]byte("legacy"))),
		ContentLength: aws.Int64(6),
	}}
	store, _ := newS3WithClient(S3Config{Bucket: "artifacts"}, client)
	object, err := store.Open(context.Background(), "legacy/object")
	if err != nil {
		t.Fatal(err)
	}
	defer object.Reader.Close()
	if object.Info.ChecksumSHA256 != "" {
		t.Fatalf("checksum=%q, want empty when endpoint omits checksum header", object.Info.ChecksumSHA256)
	}
}

func TestS3PutHonorsBoundedOperationTimeout(t *testing.T) {
	client := &fakeS3Client{waitForPutContext: true}
	store, _ := newS3WithClient(S3Config{Bucket: "artifacts", OperationTimeout: 10 * time.Millisecond}, client)
	_, err := store.Put(context.Background(), "timeout/object", bytes.NewReader([]byte("x")), PutOptions{ExpectedSize: 1})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error=%v, want context deadline exceeded", err)
	}
}

func TestS3AWSClientStreamsToCompatibleEndpoint(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	var requests atomic.Int32
	var received []byte
	client := httpClientFunc(func(request *http.Request) (*http.Response, error) {
		requests.Add(1)
		if request.Method != http.MethodPut {
			return &http.Response{StatusCode: http.StatusMethodNotAllowed, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		received = body
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
	})

	store, err := NewS3(context.Background(), S3Config{
		Bucket:           "artifacts",
		Region:           "ap-southeast-1",
		Endpoint:         "https://storage.test",
		UsePathStyle:     true,
		OperationTimeout: time.Second,
		HTTPClient:       client,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("stream through AWS SDK")
	info, err := store.Put(context.Background(), "evidence/sdk.pdf", bytes.NewReader(payload), PutOptions{ExpectedSize: int64(len(payload)), ContentType: "application/pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 || !bytes.Equal(received, payload) || info.Size != int64(len(payload)) {
		t.Fatalf("requests=%d received=%q info=%#v", requests.Load(), received, info)
	}
}

func TestS3AWSClientUsesBoundedRetriesForRetryableResponses(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	var requests atomic.Int32
	client := httpClientFunc(func(request *http.Request) (*http.Response, error) {
		attempt := requests.Add(1)
		if _, err := io.Copy(io.Discard, request.Body); err != nil {
			return nil, err
		}
		status := http.StatusOK
		if attempt < 3 {
			status = http.StatusInternalServerError
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
	})
	store, err := NewS3(context.Background(), S3Config{
		Bucket:           "artifacts",
		Region:           "ap-southeast-1",
		Endpoint:         "https://storage.test",
		UsePathStyle:     true,
		OperationTimeout: 10 * time.Second,
		HTTPClient:       client,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("retryable stream")
	if _, err := store.Put(context.Background(), "evidence/retry.pdf", bytes.NewReader(payload), PutOptions{ExpectedSize: int64(len(payload))}); err != nil {
		t.Fatalf("retryable put failed: %v", err)
	}
	if requests.Load() != 3 {
		t.Fatalf("request attempts=%d, want 3", requests.Load())
	}
}

type httpClientFunc func(*http.Request) (*http.Response, error)

func (f httpClientFunc) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestS3OpenReturnsStreamingMetadataAndMapsNotFound(t *testing.T) {
	payload := []byte("pdf")
	checksum := sha256.Sum256(payload)
	client := &fakeS3Client{getOutput: &s3.GetObjectOutput{
		Body:           io.NopCloser(bytes.NewReader(payload)),
		ContentLength:  aws.Int64(int64(len(payload))),
		ContentType:    aws.String("application/pdf"),
		ChecksumSHA256: aws.String(base64.StdEncoding.EncodeToString(checksum[:])),
	}}
	store, _ := newS3WithClient(S3Config{Bucket: "artifacts", OperationTimeout: time.Second}, client)
	object, err := store.Open(context.Background(), "workflow/pdf.pdf")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(object.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := object.Reader.Close(); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) || object.Info.Size != int64(len(payload)) || object.Info.ChecksumSHA256 != hex.EncodeToString(checksum[:]) {
		t.Fatalf("object=%#v body=%q", object.Info, got)
	}

	client.getErr = &smithy.GenericAPIError{Code: "NoSuchKey", Message: "missing"}
	client.getOutput = nil
	_, err = store.Open(context.Background(), "workflow/missing.pdf")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("not found error=%v", err)
	}
}

func TestS3RejectsUnsafeKeysBeforeCallingClient(t *testing.T) {
	client := &fakeS3Client{}
	store, _ := newS3WithClient(S3Config{Bucket: "artifacts"}, client)
	if _, err := store.Put(context.Background(), "../escape", bytes.NewReader(nil), PutOptions{ExpectedSize: 0}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("unsafe put key error=%v", err)
	}
	if _, err := store.Open(context.Background(), "/absolute"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("unsafe open key error=%v", err)
	}
	if err := store.Delete(context.Background(), "a/../b"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("unsafe delete key error=%v", err)
	}
	if client.putInput != nil || client.getInput != nil || client.deleteInput != nil {
		t.Fatal("client called for an unsafe key")
	}
}
