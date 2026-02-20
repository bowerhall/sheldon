package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/bowerhall/sheldon/internal/logger"
)

// Client wraps MinIO client with Sheldon-specific functionality
type Client struct {
	mc          *minio.Client
	userBucket  string
	agentBucket string
}

// Config holds MinIO connection settings
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// NewClient creates a new storage client
func NewClient(cfg Config) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	c := &Client{
		mc:          mc,
		userBucket:  "sheldon-user",
		agentBucket: "sheldon-agent",
	}

	return c, nil
}

// Init creates required buckets if they don't exist
func (c *Client) Init(ctx context.Context) error {
	buckets := []string{c.userBucket, c.agentBucket}

	for _, bucket := range buckets {
		exists, err := c.mc.BucketExists(ctx, bucket)
		if err != nil {
			return fmt.Errorf("check bucket %s: %w", bucket, err)
		}

		if !exists {
			if err := c.mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return fmt.Errorf("create bucket %s: %w", bucket, err)
			}
			logger.Info("bucket created", "bucket", bucket)
		}
	}

	return nil
}

// FileInfo represents a stored file
type FileInfo struct {
	Name    string
	Size    int64
	IsDir   bool
	ModTime string
}

// Upload uploads a file to the specified bucket
func (c *Client) Upload(ctx context.Context, bucket, name string, data []byte, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := c.mc.PutObject(ctx, bucket, name, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("upload %s/%s: %w", bucket, name, err)
	}

	logger.Debug("file uploaded", "bucket", bucket, "name", name, "size", len(data))
	return nil
}

// Download downloads a file from the specified bucket
func (c *Client) Download(ctx context.Context, bucket, name string) ([]byte, error) {
	obj, err := c.mc.GetObject(ctx, bucket, name, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get %s/%s: %w", bucket, name, err)
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("read %s/%s: %w", bucket, name, err)
	}

	return data, nil
}

// List lists files in a bucket with optional prefix
func (c *Client) List(ctx context.Context, bucket, prefix string) ([]FileInfo, error) {
	var files []FileInfo

	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
	}

	for obj := range c.mc.ListObjects(ctx, bucket, opts) {
		if obj.Err != nil {
			return nil, fmt.Errorf("list %s: %w", bucket, obj.Err)
		}

		files = append(files, FileInfo{
			Name:    obj.Key,
			Size:    obj.Size,
			IsDir:   strings.HasSuffix(obj.Key, "/"),
			ModTime: obj.LastModified.Format("2006-01-02 15:04:05"),
		})
	}

	return files, nil
}

// Delete deletes a file from the specified bucket
func (c *Client) Delete(ctx context.Context, bucket, name string) error {
	if err := c.mc.RemoveObject(ctx, bucket, name, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete %s/%s: %w", bucket, name, err)
	}
	return nil
}

// UserBucket returns the user bucket name
func (c *Client) UserBucket() string {
	return c.userBucket
}

// AgentBucket returns the agent bucket name
func (c *Client) AgentBucket() string {
	return c.agentBucket
}

// Healthy checks if MinIO is reachable
func (c *Client) Healthy(ctx context.Context) bool {
	_, err := c.mc.BucketExists(ctx, c.userBucket)
	return err == nil
}
