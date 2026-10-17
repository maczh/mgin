# VictoriaLogs 日志上报

在应用 YAML 中配置 `go.log.victorialogs`，框架 `mgin.Init` 自动安装内置处理器，无需业务代码注册。地址为采集服务基地址，框架追加 `/v1/logs`。

```yaml
go:
  application:
    name: myapp
  logger:
    level: info
    outputs:
      - target: victorialogs
      # 可同时添加 target: console 或 file
  log:
    sink:
      queue: 100
      workers: 1
      shutdown: 3000
    victorialogs:
      url: https://172.16.88.119:37624
      api_token: "填写采集服务令牌"
      api_crt: |
        -----BEGIN CERTIFICATE-----
        填写完整 api.crt PEM 内容
        -----END CERTIFICATE-----
      request: true
      client: true
      timeout: 5000
```

`request` 上报 PostLog，映射为 `type: request`，原始结构保存在 `fields`。`client` 独立上报每次 client.Call 的服务、方法、路径、结果和耗时（含重试尝试），同样使用 `type: request`，不受运行日志级别过滤。运行日志在 `go.logger.outputs` 选择 `victorialogs`，使用 `type: console`，保留调用位置和 request_id，遵循 logger.level。应用名称必须符合采集接口的 1–100 字符命名规则。

`outputs` 非空时替代旧 `out/file` 配置。启用 VictoriaLogs 不会取消已配置的 MongoDB 接口日志存储；仅重定向时清空旧 `go.log.req`。client 上报的是调用摘要，不额外采集请求认证头和响应正文。

每个 handler 最多积压 100 条，单协程 FIFO 消费；满时淘汰最早待写日志，为新日志腾出空间。旧 queue 大于 100 时归一为 100，workers 统一为 1；较小 queue 仍有效。当前正在写入的日志不属于等待队列。

使用配置的 PEM 验证 TLS 证书和主机名，不跳过校验。每次请求默认超时 5 秒；503 最多尝试 3 次，间隔 100/200ms。202 表示采集服务接收成功，不代表已可查询；其他状态及网络错误记录为 handler 失败。失败不无限重试，进程退出尽力排空，内存队列不保证持久化投递。

日志正文最大 1MiB，超过时处理失败。凭据只放应用私有配置，不提交版本库。
