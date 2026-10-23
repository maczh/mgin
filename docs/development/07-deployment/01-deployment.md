# 部署形态

## 1. 二进制 + 配置文件（最简单）

```
/opt/order/
├── order                 # 二进制（工程根目录构建产物）
├── conf/
│   ├── application.yml
│   ├── mysql-prod.yml
│   └── nacos-prod.yml
└── logs/
```

```bash
cd /opt/order
./order                     # 默认找 ./order.yml
./order -f conf/application.yml
```

注意：

- `NewApp` 把**非绝对路径**拼到 `filepath.Dir(os.Args[0])`；
  以 `./order` 启动时，`conf/application.yml` → `/opt/order/conf/application.yml`；
- 通过 PATH 启动（`os.Args[0]` 只有程序名）时，相对路径等价于当前工作目录；
- `cert/key`、SQLite 文件也按同样规则解析；
- 用 systemd/supervisor 时显式设置工作目录与 `-f` 绝对路径最稳。

systemd 示例：

```ini
[Unit]
Description=order service
After=network.target

[Service]
WorkingDirectory=/opt/order
ExecStart=/opt/order/order -f /opt/order/conf/application.yml
Restart=on-failure
RestartSec=3
KillSignal=SIGTERM
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
```

---

## 2. Docker

```dockerfile
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 使用 SQLite 时必须 CGO_ENABLED=1 且安装 gcc
RUN CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /out/order .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/order /app/order
COPY conf /app/conf
EXPOSE 8080
ENTRYPOINT ["/app/order", "-f", "/app/conf/application.yml"]
```

构建与运行：

```bash
docker build -t order:1.0.0 .
docker run -d --name order -p 8080:8080 \
  -v $PWD/conf:/app/conf:ro \
  -e TZ=Asia/Shanghai \
  order:1.0.0
```

要点：

- 配置不要打进镜像（或用只读挂载覆盖），便于按环境替换；
- 日志目录挂载卷，避免容器重建丢日志；
- 资源限制：`--memory`、`--cpus`；Go 1.25 会自动识别 cgroup 限额；
- 时区：`TZ` + 镜像内 tzdata，否则定时任务时区可能不符。

---

## 3. Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order
spec:
  replicas: 2
  selector:
    matchLabels: { app: order }
  template:
    metadata:
      labels: { app: order }
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      terminationGracePeriodSeconds: 30     # 必须 > shutdownTimeout
      containers:
        - name: order
          image: registry/order:1.0.0
          args: ["-f", "/app/conf/application.yml"]
          ports:
            - { name: http, containerPort: 8080 }
          env:
            - { name: TZ, value: Asia/Shanghai }
          resources:
            requests: { cpu: "200m", memory: "256Mi" }
            limits:   { cpu: "2",    memory: "1Gi" }
          startupProbe:
            httpGet: { path: /health/startup, port: http }
            failureThreshold: 30
            periodSeconds: 2
          livenessProbe:
            httpGet: { path: /health/live, port: http }
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 2
            failureThreshold: 3
          readinessProbe:
            httpGet: { path: /health/ready, port: http }
            periodSeconds: 5
            timeoutSeconds: 3      # 必须大于最慢一次依赖检查
            failureThreshold: 2
          volumeMounts:
            - { name: conf, mountPath: /app/conf, readOnly: true }
      volumes:
        - name: conf
          configMap: { name: order-conf }
```

### 3.1 探针为什么这样配

| 探针 | 行为 | 配置要点 |
|------|------|----------|
| startup | 未 `MarkStarted` 前 503 | `failureThreshold` 覆盖最慢启动时间 |
| liveness | 恒 200（不查依赖） | 不要用 `/health/ready` 做 liveness，否则依赖抖动会重启全部实例 |
| readiness | 实时 Check MySQL/Mongo/Redis | `timeoutSeconds` > 最慢检查；失败摘流量但不重启 |

### 3.2 配置下发

- ConfigMap/Secret 挂载 `conf/`，变更后滚动重启（框架不监听文件变更）；
- 敏感信息用 Secret，不要放 ConfigMap；
- 多环境用 Kustomize/Helm 覆盖 `go.config.env` 与地址。

---

## 4. 多副本约束

| 能力 | 多副本行为 | 建议 |
|------|-----------|------|
| HTTP 服务 | 正常，需 LB/Service | — |
| 注册中心 | 每个副本注册一次 | 确认实例 ID 含 IP:Port，避免互相覆盖 |
| 服务发现缓存 | 各副本独立 | 可接受 |
| 定时任务 | **每个启用 job 的副本都会调度** | 只让一个副本 `go.job.enabled=true`，或加选主 |
| 限流 | 每副本独立计数 | 实际放行 ≈ 单副本阈值 × 副本数 |
| 会话 | 内存会话不共享 | 用 Redis 存储或粘性会话 |
| 接口日志 | 各副本各自写 | Mongo/VL 端按 requestId 聚合 |
| JWT | 无状态，天然支持 | secret 必须一致 |

---

## 5. HTTPS

```yaml
go:
  application:
    port_ssl: 8443
    cert: certs/server.crt
    key: certs/server.key
```

- 同时监听 HTTP 与 HTTPS（不是二选一）；
- 证书相对可执行文件目录；
- 不支持证书热更新与 mTLS；生产通常在 Ingress/Mesh 终止 TLS。

---

## 6. 发布方式

| 方式 | 适用 | 注意 |
|------|------|------|
| 滚动更新 | 无状态服务 | `maxUnavailable: 0` + readiness 保证不中断 |
| 蓝绿 | 需要快速回滚 | 两套注册分组，切换流量 |
| 金丝雀 | 风险敏感 | 框架无权重路由，用网关/Mesh 或独立服务名 |
| 就地替换 | 单机 | 先摘流量再停进程，等待优雅关闭 |

发布前检查：[生产就绪清单](02-operations.md#1-生产就绪清单)。
