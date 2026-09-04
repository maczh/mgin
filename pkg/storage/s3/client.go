package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/maczh/mgin/v2/pkg/logs"
)

// S3 是 S3 存储插件，实现 mgin.MginPlugin 接口。
//
// 用法：
//
//	mgin.UsePlugin("s3", storage.NewS3())
//	// 或手动初始化：
//	s := storage.NewS3()
//	s.Init(configBytes) // configBytes 为 go.s3 节点的 yaml 字节
//
// 业务侧取桶：
//
//	b := storage.GetS3().Default()       // 默认桶
//	b := storage.GetS3().Get("avatar")   // 指定桶
//	b.Upload(ctx, "users/1.png", data, "image/png")
type S3 struct {
	cfg      *Config
	awsCfg   aws.Config
	buckets  map[string]*Bucket
	order    []string
	clientMu sync.RWMutex
}

var (
	instance *S3
	once     sync.Once
)

// NewS3 创建（或返回单例）S3 存储插件
func NewS3() *S3 {
	once.Do(func() {
		instance = &S3{buckets: make(map[string]*Bucket)}
	})
	return instance
}

// GetS3 返回全局 S3 插件单例
func GetS3() *S3 {
	return NewS3()
}

// Init 实现 mgin.MginPlugin 接口，解析配置并建立客户端。
// configData 为 go.s3 节点对应的 yaml 字节。
func (s *S3) Init(configData []byte) {
	if len(configData) == 0 {
		logs.Error("[S3] 配置为空，未初始化")
		return
	}
	cfg := &Config{}
	if err := unmarshalYAML(configData, cfg); err != nil {
		logs.Error("[S3] 配置解析失败: {}", err.Error())
		return
	}
	cfg.normalize()
	if cfg.Endpoint == "" {
		logs.Error("[S3] endpoint 未配置，S3 插件未初始化")
		return
	}
	s.cfg = cfg

	// 构造 aws.Config
	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithRetryMaxAttempts(cfg.MaxRetries),
	}
	if cfg.AccessKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, cfg.SessionToken)))
	}
	awscfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		logs.Error("[S3] AWS 配置加载失败: {}", err.Error())
		return
	}
	s.awsCfg = awscfg

	// 构建桶客户端
	bucketNames := make([]string, 0)
	if len(cfg.Buckets) == 0 {
		if cfg.SingleBucket == "" {
			logs.Error("[S3] 未配置 buckets 也未配置 singleBucket，S3 插件未初始化")
			return
		}
		bucketNames = append(bucketNames, cfg.SingleBucket)
	} else {
		for _, b := range cfg.Buckets {
			if b.Name != "" {
				bucketNames = append(bucketNames, b.Name)
			}
		}
	}
	for _, name := range bucketNames {
		b := s.newBucket(name)
		s.buckets[name] = b
		s.order = append(s.order, name)
	}
	logs.Info("[S3] 存储插件已初始化, endpoint={}, 桶={}", cfg.Endpoint, strings.Join(bucketNames, ","))
}

// newBucket 为指定桶名构造客户端与上传/下载管理器
func (s *S3) newBucket(name string) *Bucket {
	defaultCT := ""
	pathStyle := s.cfg.pathStyle()
	for _, b := range s.cfg.Buckets {
		if b.Name == name {
			defaultCT = b.DefaultContentType
		}
	}
	// 共享底层 client，按 pathStyle 决定是否路径风格
	client := s3.NewFromConfig(s.awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(s.cfg.Endpoint)
		o.UsePathStyle = pathStyle
		o.Region = s.cfg.Region
	})
	return &Bucket{
		name:      name,
		defaultCT: defaultCT,
		client:    client,
		presign:   s3.NewPresignClient(client),
		uploader: manager.NewUploader(client, func(u *manager.Uploader) {
			u.PartSize = s.cfg.UploadPartSize
			u.MaxUploadParts = int32(s.cfg.MaxUploadParts)
		}),
		downloader: manager.NewDownloader(client, func(d *manager.Downloader) {
			d.PartSize = s.cfg.DownloadPartSize
			d.Concurrency = s.cfg.MaxDownloadParts
		}),
		partSize: s.cfg.UploadPartSize,
		sem:      make(chan struct{}, 64),
	}
}

// Default 返回默认桶（buckets 第一个，或 singleBucket）
func (s *S3) Default() *Bucket {
	s.clientMu.RLock()
	defer s.clientMu.RUnlock()
	if len(s.order) == 0 {
		return nil
	}
	return s.buckets[s.order[0]]
}

// Get 按桶名返回桶客户端，不存在返回 nil
func (s *S3) Get(name string) *Bucket {
	s.clientMu.RLock()
	defer s.clientMu.RUnlock()
	return s.buckets[name]
}

// Close 实现 mgin.MginPlugin 接口。S3 客户端无持久连接，此处仅做清理。
func (s *S3) Close() {
	s.clientMu.Lock()
	s.buckets = make(map[string]*Bucket)
	s.order = nil
	s.clientMu.Unlock()
	logs.Info("[S3] 存储插件已关闭")
}

// Check 实现 mgin.MginPlugin 接口，对默认桶做一次 HeadBucket 探活。
func (s *S3) Check() error {
	b := s.Default()
	if b == nil {
		return errors.New("S3 未初始化")
	}
	return b.Check(context.Background())
}

// ---------------------------------------------------------------- Bucket

// Bucket 代表一个 S3 桶的客户端，提供上传/下载/删除/列举/预签名/分片上传等操作。
type Bucket struct {
	name       string
	defaultCT  string
	client     *s3.Client
	presign    *s3.PresignClient
	uploader   *manager.Uploader
	downloader *manager.Downloader
	partSize   int64
	sem        chan struct{}
}

// Name 返回桶名
func (b *Bucket) Name() string { return b.name }

// Check 探活
func (b *Bucket) Check(ctx context.Context) error {
	_, err := b.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(b.name)})
	return err
}

// PutObject / Put 别名：上传字节
func (b *Bucket) Put(ctx context.Context, key string, data []byte, contentType string) error {
	return b.Upload(ctx, key, bytes.NewReader(data), contentType)
}

// Upload 上传一个 Reader 内容到指定 key。
// contentType 为空时使用桶的默认类型，再为空时按扩展名推断，仍为空则 application/octet-stream。
// 对于 *bytes.Reader 会自动判断是否超过单片大小从而选用分片上传；其他 Reader 走 PutObject。
func (b *Bucket) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	if contentType == "" {
		contentType = b.resolveContentType(key)
	}
	input := &s3.PutObjectInput{
		Bucket:      aws.String(b.name),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}
	// 仅当 body 为 *bytes.Reader 时可安全预估大小（不消费 body）
	if br, ok := body.(*bytes.Reader); ok && int64(br.Len()) > b.partSize {
		_, err := b.uploader.Upload(ctx, input)
		return wrapErr(err)
	}
	_, err := b.client.PutObject(ctx, input)
	return wrapErr(err)
}

// UploadBytes 便捷方法：直接上传字节切片
func (b *Bucket) UploadBytes(ctx context.Context, key string, data []byte, contentType string) error {
	return b.Put(ctx, key, data, contentType)
}

// Download 将指定 key 的内容写入 w（io.Writer），大文件自动分片下载。
func (b *Bucket) Download(ctx context.Context, key string, w io.WriterAt) error {
	_, err := b.downloader.Download(ctx, w, &s3.GetObjectInput{
		Bucket: aws.String(b.name),
		Key:    aws.String(key),
	})
	return wrapErr(err)
}

// DownloadBytes 下载指定 key 的完整内容并返回字节切片。
func (b *Bucket) DownloadBytes(ctx context.Context, key string) ([]byte, error) {
	buf := manager.NewWriteAtBuffer([]byte{})
	if err := b.Download(ctx, key, buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Delete 删除指定 key，不存在不报错。
func (b *Bucket) Delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.name),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil
		}
		return wrapErr(err)
	}
	return nil
}

// Exists 判断 key 是否存在
func (b *Bucket) Exists(ctx context.Context, key string) (bool, error) {
	_, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.name),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsf *types.NotFound
		if errors.As(err, &nsf) {
			return false, nil
		}
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return false, nil
		}
		return false, wrapErr(err)
	}
	return true, nil
}

// List 列举桶内对象，prefix 为前缀过滤，max 为最大返回数量（<=0 取 1000）。
// 返回的 ObjectInfo 含 Key/Size/LastModified/ETag/ContentType。
func (b *Bucket) List(ctx context.Context, prefix string, max int32) ([]ObjectInfo, error) {
	if max <= 0 {
		max = 1000
	}
	out, err := b.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(b.name),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(max),
	})
	if err != nil {
		return nil, wrapErr(err)
	}
	list := make([]ObjectInfo, 0, len(out.Contents))
	for _, o := range out.Contents {
		info := ObjectInfo{Key: aws.ToString(o.Key), Size: aws.ToInt64(o.Size)}
		if o.LastModified != nil {
			info.LastModified = *o.LastModified
		}
		info.ETag = aws.ToString(o.ETag)
		list = append(list, info)
	}
	return list, nil
}

// Presign 生成下载预签名 URL，有效期使用配置的 presignExpiry。
func (b *Bucket) Presign(ctx context.Context, key string) (string, error) {
	req, err := b.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.name),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(GetS3().cfg.presignDuration()))
	if err != nil {
		return "", wrapErr(err)
	}
	return req.URL, nil
}

// PresignUpload 生成上传预签名 URL（客户端直传）。
func (b *Bucket) PresignUpload(ctx context.Context, key string) (string, error) {
	req, err := b.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(b.name),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(GetS3().cfg.presignDuration()))
	if err != nil {
		return "", wrapErr(err)
	}
	return req.URL, nil
}

// UploadMultipart 分片上传（适合超大文件）。
// 返回最终 ETag。body 可分多次写入通过 partSize 控制单片大小。
func (b *Bucket) UploadMultipart(ctx context.Context, key, contentType string, body io.Reader, partSize int64) (string, error) {
	if contentType == "" {
		contentType = b.resolveContentType(key)
	}
	if partSize <= 0 {
		partSize = b.partSize
	}
	// 1) 创建分片任务
	co, err := b.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(b.name),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", wrapErr(err)
	}
	uploadID := aws.ToString(co.UploadId)
	parts := make([]types.CompletedPart, 0)
	buf := make([]byte, partSize)
	partNum := int32(1)
	defer func() {
		// 异常时中止分片任务，避免产生碎片
		if err != nil {
			_, _ = b.client.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(b.name),
				Key:      aws.String(key),
				UploadId: aws.String(uploadID),
			})
		}
	}()
	for {
		n, readErr := io.ReadFull(body, buf)
		if n > 0 {
			partBody := buf[:n]
			out, perr := b.client.UploadPart(ctx, &s3.UploadPartInput{
				Bucket:     aws.String(b.name),
				Key:        aws.String(key),
				UploadId:   aws.String(uploadID),
				PartNumber: aws.Int32(partNum),
				Body:       bytes.NewReader(partBody),
			})
			if perr != nil {
				err = perr
				return "", wrapErr(perr)
			}
			parts = append(parts, types.CompletedPart{
				ETag:       out.ETag,
				PartNumber: aws.Int32(partNum),
			})
			partNum++
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil && readErr != io.ErrUnexpectedEOF {
			err = readErr
			return "", wrapErr(readErr)
		}
	}
	// 2) 完成分片任务
	res, cerr := b.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(b.name),
		Key:             aws.String(key),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	})
	if cerr != nil {
		err = cerr
		return "", wrapErr(cerr)
	}
	return aws.ToString(res.ETag), nil
}

// resolveContentType 推断内容类型
func (b *Bucket) resolveContentType(key string) string {
	if b.defaultCT != "" {
		return b.defaultCT
	}
	ext := strings.ToLower(filepath.Ext(key))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".zip":
		return "application/zip"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	default:
		return "application/octet-stream"
	}
}

// ObjectInfo 列举返回的对象摘要
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	ContentType  string
}

// wrapErr 将存储错误原样返回（统一收敛点，便于后续扩展错误包装）
func wrapErr(err error) error {
	if err == nil {
		return nil
	}
	return err
}
