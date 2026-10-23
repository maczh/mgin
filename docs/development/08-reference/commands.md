# 命令参考

## 1. 脚手架 `mgin`

### 1.1 构建

```bash
cd cmd
go build -o bin/mgin .
./bin/mgin version
```

`cmd` 是独立 module（`github.com/maczh/mgin/cmd/mgin`），需在 `cmd/` 目录构建。

### 1.2 语法

```
mgin new <project-name> [options]
```

| 选项 | 默认 | 说明 |
|------|------|------|
| `--module <path>` | `github.com/maczh/<name>` | Go module 路径 |
| `--port <n>` | 8080 | 服务端口 |
| `--db <type>` | `mysql` | `mysql/sqlite/mongodb/redis/postgres/clickhouse/elasticsearch` |
| `--mq <type>` | 空 | `nats/kafka/mqtt/rabbit`（仅在生成配置/依赖时使用） |
| `--registry <type>` | 空 | `nacos/consul/etcd/polaris` |
| `--config-center <type>` | 空 | 配置中心类型 |
| `--i18n` | false | 启用国际化骨架 |
| `--jwt` | false | 启用 JWT 中间件骨架 |
| `--casbin` | false | 启用 Casbin 骨架 |
| `--sys` | false | 仅 master 分支；jh 已移除系统管理模块 |
| `--mgin-version <v>` | 最新发布版 | 覆盖依赖版本 |
| `--output <dir>` | `.` | 输出目录 |
| `--force` | false | 覆盖已存在目录 |

### 1.3 示例

```bash
# 最小工程
./bin/mgin new demo --module example.com/demo --port 8080 --output /tmp

# 带 MySQL + Nacos + JWT
./bin/mgin new order --module example.com/order --port 9000 \
  --db mysql --registry nacos --jwt --output /tmp

# 指定框架版本（离线环境）
./bin/mgin new demo --mgin-version v1.25.27-jh --output /tmp
```

生成后：

```bash
cd /tmp/demo
go mod tidy
go build -o demo .
./demo
```

> `--db postgres|clickhouse` 会生成不存在的 `dao.PostgresDao`/`dao.ClickhouseDao`，工程无法编译。`--db sqlite/mongodb/redis/elasticsearch` 不生成 `dao/`，业务层为内存示例。

### 1.4 生成的 Makefile

| 目标 | 作用 |
|------|------|
| `make tidy` | `go mod tidy` |
| `make build` | 构建当前平台，注入版本/BuildTime/GitHash |
| `make linux` | 交叉编译 Linux amd64 |
| `make build-multi-os` | 多平台构建 |
| `make run` | 构建并运行 |
| `make test` | `go test ./...` |
| `make test-race` | `go test -race ./...` |
| `make cover` | 覆盖率并生成 HTML |
| `make lint` | `gofmt` + `go vet` |
| `make clean` | 清理产物 |

---

## 2. Go 工具链

```bash
go version                       # 需要 1.25.7
go mod download
go build ./...                   # 框架本体
cd cmd && go build ./...         # 脚手架
gofmt -l $(git ls-files '*.go' | grep -v '^.Trash-0/')
go vet ./...
go test ./health/ ./metrics/ ./logs/ ./logsink/ ./utils/ ./middleware/cors/
go test -race ./logsink/ ./cache/ ./utils/
go test -coverprofile=cover.out ./... && go tool cover -html=cover.out
```

`go vet ./...` 目前仍有 3 处历史告警（`middleware/xss`、`utils/rsautil.go` 的 unreachable code；`i18n/store_test.go` 的 copylocks），不影响构建。

---

## 3. 运行时命令

```bash
# 探针
curl -s localhost:8080/health/live | jq .
curl -s -w '\n%{http_code}\n' localhost:8080/health/ready
curl -s localhost:8080/health/startup

# 指标
curl -s localhost:8080/metrics | grep mgin_http_requests_total

# 配置重载（需 go.config.reload.token）
curl -s -X PUT localhost:8080/cfg/reload \
  -H "X-MGin-Reload-Token: $TOKEN"

# 任务
curl -s localhost:8080/job/list
curl -s localhost:8080/job/handlers
curl -s -X POST localhost:8080/job/1/trigger
curl -s "localhost:8080/job/log?jobId=1&index=1&size=20"

# 字典（需 go.i18n.api.enabled）
curl -s "localhost:8080/i18n/list?page=1&pageSize=20"
```

---

## 4. 本地中间件

```bash
docker run -d --name mgin-mysql -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=demo mysql:8.0
docker run -d --name mgin-redis -p 6379:6379 redis:7-alpine
docker run -d --name mgin-mongo -p 27017:27017 mongo:6

docker logs -f mgin-mysql
docker exec -it mgin-mysql mysql -uroot -proot -e 'show databases;'
```

---

## 5. 进程与端口

```bash
ss -lntp | grep :8080
ps -ef | grep '[d]emo'
kill -TERM <pid>          # 触发优雅关闭
journalctl -u order -f    # systemd 日志
```

---

## 6. 常用诊断

```bash
# 配置是否被读取
grep -n "读取配置文件" logs/app-*.log | tail

# 依赖状态
curl -s localhost:8080/health/ready | jq .dependencies

# 请求链路
grep '<request-id>' logs/app-$(date +%F).log

# MongoDB 接口日志
mongosh --quiet --eval 'db.demoRequestLog.find().sort({time:-1}).limit(1).pretty()'

# 版本信息
curl -s localhost:8080/health/live | jq .data.version
```
