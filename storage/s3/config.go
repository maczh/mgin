package s3

import "time"

// Config 是 S3 存储插件的完整配置，对应 application.yml 中 go.s3 节点。
//
// 多 bucket：buckets 为空时自动以 endpoint 对应的默认桶(singleBucket)工作；
// buckets 非空时通过 Get(name) 区分不同用途的桶（如公有资源桶 / 私有文件桶）。
type Config struct {
	// Enabled 是否启用（go.config.used 已包含 s3 时仍可用此开关二次控制）
	Enabled bool `yaml:"enabled"`
	// Endpoint S3 兼容服务地址，如 https://s3.amazonaws.com 或 http://localhost:9000（MinIO）
	Endpoint string `yaml:"endpoint"`
	// Region 区域，如 cn-north-1 / us-east-1，兼容服务可填 auto
	Region string `yaml:"region"`
	// AccessKey 访问密钥 ID
	AccessKey string `yaml:"accessKey"`
	// SecretKey 访问密钥
	SecretKey string `yaml:"secretKey"`
	// SessionToken 可选，临时凭证的会话令牌
	SessionToken string `yaml:"sessionToken"`
	// PathStyle 是否使用路径风格寻址（MinIO 等自建服务必须为 true）
	PathStyle bool `yaml:"pathStyle"`
	// ForcePathStyle 的别名，等效于 PathStyle
	ForcePathStyle bool `yaml:"forcePathStyle"`
	// UsePathStyle 的别名，等效于 PathStyle
	UsePathStyle bool `yaml:"usePathStyle"`
	// SSL 是否启用 HTTPS，默认 true
	SSL bool `yaml:"ssl"`
	// MaxRetries 请求失败重试次数，默认 3
	MaxRetries int `yaml:"maxRetries"`
	// UploadPartSize 分片上传的单片大小（字节），默认 16MB
	UploadPartSize int64 `yaml:"uploadPartSize"`
	// DownloadPartSize 分片下载的单片大小（字节），默认 16MB
	DownloadPartSize int64 `yaml:"downloadPartSize"`
	// MaxUploadParts 分片上传最大并发数，默认 10
	MaxUploadParts int `yaml:"maxUploadParts"`
	// MaxDownloadParts 分片下载最大并发数，默认 10
	MaxDownloadParts int `yaml:"maxDownloadParts"`
	// PresignExpiry 预签名 URL 默认有效期（秒），默认 3600
	PresignExpiry int `yaml:"presignExpiry"`
	// SingleBucket 未配置 buckets 时使用的默认桶名
	SingleBucket string `yaml:"singleBucket"`
	// Buckets 多桶配置
	Buckets []BucketConfig `yaml:"buckets"`
}

// BucketConfig 单个桶的配置
type BucketConfig struct {
	// Name 桶名（必填）
	Name string `yaml:"name"`
	// Public 是否为公有桶（用于预签名默认行为提示，不影响实际 ACL）
	Public bool `yaml:"public"`
	// DefaultContentType 该桶上传时未指定 ContentType 时的默认值
	DefaultContentType string `yaml:"defaultContentType"`
}

// normalize 填充默认值
func (c *Config) normalize() {
	if c.Region == "" {
		c.Region = "us-east-1"
	}
	if c.SSL && (c.Endpoint == "" || !isHTTP(c.Endpoint)) {
		// SSL 仅作语义标记，实际由 endpoint 协议决定；默认开启
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	if c.UploadPartSize <= 0 {
		c.UploadPartSize = 16 * 1024 * 1024
	}
	if c.DownloadPartSize <= 0 {
		c.DownloadPartSize = 16 * 1024 * 1024
	}
	if c.MaxUploadParts <= 0 {
		c.MaxUploadParts = 10
	}
	if c.MaxDownloadParts <= 0 {
		c.MaxDownloadParts = 10
	}
	if c.PresignExpiry <= 0 {
		c.PresignExpiry = 3600
	}
}

// presignDuration 返回预签名有效期
func (c *Config) presignDuration() time.Duration {
	return time.Duration(c.PresignExpiry) * time.Second
}

// pathStyle 综合三种字段判定是否路径风格
func (c *Config) pathStyle() bool {
	return c.PathStyle || c.ForcePathStyle || c.UsePathStyle
}

// isHTTP 仅判断字符串是否以 http:// 或 https:// 开头
func isHTTP(s string) bool {
	return len(s) >= 7 && (s[:7] == "http://" || (len(s) >= 8 && s[:8] == "https://"))
}
