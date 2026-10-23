# 部署

## 1. 产物

```bash
# 交叉编译（脚手架 Makefile 已包含）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags "-s -w -X main.Version=$VERSION -X main.BuildTime=$(date -u +%FT%TZ) -X main.GitHash=$(git rev-parse --short HEAD)" \
  -o demo .
```

- SQLite 使用纯 Go 驱动，`CGO_ENABLED=0` 可静态编译。
- `version.go` 的变量必须是包级 `string`，否则 `-X` 无效。
- 二进制 + `conf/` 即最小部署单元。

## 2. 配置准备

```yaml
go:
  application:
    name: order-service
    port: 8080
    project: order
    debug: false
    ip: 10.0.1.12        # 多网卡/容器网络建议显式指定
  logger:
    level: info
    outputs:
      - target: console
        format: json
      - target: file
        file: /var/log/order/app
  config:
    used: "mysql,redis,nacos"
    env: prod
```

检查清单：

- [ ] `debug: false`
- [ ] `go.application.ip` 与注册中心可达地址一致
- [ ] 子配置文件名与 `prefix`/`env` 匹配
- [ ] 密码/密钥来自 Secret，不落仓库
- [ ] 日志目录可写、磁盘有配额

## 3. 单机 / 虚拟机

```bash
mkdir -p /opt/order/{bin,conf,logs}
cp demo /opt/order/bin/ && cp conf/*.yml /opt/order/conf/
/opt/order/bin/demo -f /opt/order/conf/application.yml
```

建议用 systemd 管理：

```ini
[Service]
WorkingDirectory=/opt/order
ExecStart=/opt/order/bin/demo -f /opt/order/conf/application.yml
Restart=always
RestartSec=3
KillSignal=SIGTERM
TimeoutStopSec=15
Environment=GOMAXPROCS=4
```

`KillSignal=SIGTERM` 对应框架的优雅关闭；`TimeoutStopSec` 要大于「排空日志 + 关闭 HTTP（5s）」的总时间。

## 4. 容器

```dockerfile
FROM golang:1.25.7 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/app .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/app /app/app
COPY conf /app/conf
EXPOSE 8080
ENTRYPOINT ["/app/app", "-f", "/app/conf/application.yml"]
```

注意：

- 工作目录固定，配置用绝对路径。
- distroless 没有 shell，排障用 sidecar 或临时换基础镜像。
- 容器内 `go.application.ip` 常需设为 Pod IP 或让注册中心使用容器网络地址。

## 5. Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order-service
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: app
          image: registry/order-service:1.2.0
          args: ["-f", "/app/conf/application.yml"]
          ports: [{ containerPort: 8080 }]
          readinessProbe:
            httpGet: { path: /health/ready, port: 8080 }
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet: { path: /health/live, port: 8080 }
            initialDelaySeconds: 10
            periodSeconds: 20
          startupProbe:
            httpGet: { path: /health/startup, port: 8080 }
            failureThreshold: 30
            periodSeconds: 2
          lifecycle:
            preStop:
              exec: { command: ["sleep", "5"] }
          resources:
            requests: { cpu: 200m, memory: 256Mi }
            limits:   { cpu: "1",  memory: 512Mi }
```

要点：

- `startupProbe` 用 `/health/startup`：未调用 `MarkStarted` 时返回 503，避免初始化未完成就接流量。
- `readinessProbe` 用 `/health/ready`：它会实时检查 MySQL/Mongo/Redis。
- `preStop` 睡眠给 Service 摘流留时间，再触发 SIGTERM。
- 多副本时 `job` 只能在一个副本启用（单机调度）。

## 6. 优雅停机顺序

```
SIGTERM → SafeExit()
  ├─ postlog.Close()        排空接口日志
  ├─ i18n.Close()
  ├─ 插件 Close()
  ├─ registry.DeRegister()  注销实例
  ├─ db.*.Close()           关闭连接
  └─ http.Server.Shutdown(5s) 等待在途请求
```

调大 `terminationGracePeriodSeconds`（≥ 20）以覆盖排空时间。

## 7. 资源与容量

| 资源 | 关注点 |
|------|--------|
| CPU | JSON 序列化与日志格式化是主要开销；`GOMAXPROCS` 与 limit 对齐 |
| 内存 | 内存缓存无上限、日志队列有界；大响应体会推高 GC |
| 连接 | `mysql_pool.max` × 副本数 ≤ 数据库上限的 70% |
| 文件 | 日志按天增长不清理；需配额或 logrotate |
| 端口 | HTTP/HTTPS 同时监听时注意安全组 |

## 8. 发布与回滚

- 镜像 tag 使用不可变版本号（不要 `latest`）。
- 滚动更新时 `maxUnavailable: 0`，保证探针通过后再接流量。
- 回滚：`kubectl rollout undo` 或切回上一个镜像 tag；配置变更同步回滚。
- 数据库变更遵循「只加不删」，与代码解耦发布。
