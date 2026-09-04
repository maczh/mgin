package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode/utf8"
)

// scaffold 推导配置并写出整个工程骨架。
func scaffold(opts *ProjectOptions) error {
	computeDerived(opts)

	root := opts.OutputDir
	if root == "" {
		root = "."
	}
	root = filepath.Join(root, opts.ProjectName)

	if info, err := os.Stat(root); err == nil && info.IsDir() {
		if !opts.Force {
			return fmt.Errorf("目录 %s 已存在, 使用 --force 覆盖", root)
		}
	}

	created := make([]string, 0)

	write := func(rel string, content string) error {
		// 保证生成的源码(含中文注释)均为合法 UTF-8 编码, 杜绝乱码。
		if !utf8.ValidString(content) {
			return fmt.Errorf("生成的文件 %s 含非法 UTF-8 字符, 已中止写入", rel)
		}
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			return err
		}
		created = append(created, rel)
		return nil
	}

	// 主程序与业务代码
	if err := write("main.go", render(tmplMain, opts)); err != nil {
		return err
	}
	// 版本信息文件(Makefile 通过 -ldflags 注入 Version/BuildTime/GitHash)
	if err := write("version.go", render(tmplVersion, opts)); err != nil {
		return err
	}
	// 工程构建脚本
	makefile := strings.ReplaceAll(tmplMakefile, "{{.ProjectName}}", opts.ProjectName)
	if err := write("Makefile", makefile); err != nil {
		return err
	}
	// 消息队列插件注册(多选时一次性注册全部)
	if opts.HasMQ {
		if err := write("plugins.go", render(tmplPlugins, opts)); err != nil {
			return err
		}
	}
	if err := write("router/router.go", render(tmplRouter, opts)); err != nil {
		return err
	}
	if err := write("controller/controller.go", render(tmplController, opts)); err != nil {
		return err
	}
	if err := write("model/model.go", render(tmplModel, opts)); err != nil {
		return err
	}
	if opts.DataLayer == "memory" {
		if err := write("service/service.go", render(tmplServiceMemory, opts)); err != nil {
			return err
		}
	} else {
		if err := write("service/service.go", render(tmplServiceDB, opts)); err != nil {
			return err
		}
		if err := write("dao/dao.go", render(tmplDao, opts)); err != nil {
			return err
		}
	}

	// 配置文件（统一放在 conf/ 目录下: application.yml + 各组件 <prefix>-<env>.yml）
	if err := write("conf/application.yml", buildAppYAML(opts)); err != nil {
		return err
	}
	for _, comp := range opts.Components {
		meta := componentMeta[comp]
		if !meta.yamlFile {
			continue
		}
		tmpl, ok := componentTemplates[comp]
		if !ok {
			continue
		}
		prefix := meta.prefix
		if comp == "sqlite" {
			prefix = opts.ProjectName + ".db"
		}
		content := render(tmpl, opts)
		if err := write("conf/"+buildComponentFileName(prefix, opts.Env), content); err != nil {
			return err
		}
	}
	if opts.Casbin {
		if err := write("conf/casbin.conf", casbinModel); err != nil {
			return err
		}
	}

	// go.mod 与说明文档
	if err := write("go.mod", render(tmplGoMod, opts)); err != nil {
		return err
	}
	if err := write("README.md", buildReadme(opts)); err != nil {
		return err
	}
	if err := write(".gitignore", "conf/*.db\nlogs/\n"+opts.ProjectName+"\n"); err != nil {
		return err
	}

	fmt.Printf("\n✅ 已在 %s 创建工程骨架 (%s)\n\n", root, opts.Module)
	fmt.Println("生成的文件:")
	for _, f := range created {
		fmt.Printf("  - %s\n", f)
	}
	fmt.Println("\n下一步:")
	fmt.Println("  cd " + root)
	fmt.Println("  go mod tidy")
	fmt.Printf("  go build -o %s .\n", opts.ProjectName)
	fmt.Printf("  ./%s            # 读取 conf/application.yml\n", opts.ProjectName)
	return nil
}

// HasMQKey 判断是否选择了指定的消息队列(忽略大小写)。
func (o *ProjectOptions) HasMQKey(key string) bool {
	for _, m := range o.MQ {
		if strings.EqualFold(strings.TrimSpace(m), key) {
			return true
		}
	}
	return false
}

// computeDerived 根据用户输入推导 used/prefix/数据层/配置中心等字段。
func computeDerived(o *ProjectOptions) {
	o.Env = "test"
	o.BaseURI = "/api/v1"

	comps := make([]string, 0)
	seen := map[string]bool{}
	add := func(c string) {
		if c == "" || seen[c] || c == "none" {
			return
		}
		if _, ok := componentMeta[c]; !ok {
			return
		}
		seen[c] = true
		comps = append(comps, c)
	}

	for _, db := range o.DBs {
		add(db)
	}
	// 消息队列支持多选, 按固定顺序注册, 保证 go.mod / plugins.go 输出稳定
	o.MQPlugins = nil
	for _, key := range mqOrder {
		if !o.HasMQKey(key) {
			continue
		}
		add(key)
		p := mqPlugins[key]
		o.MQPlugins = append(o.MQPlugins, MQPlugin{
			Name:      key,
			Path:      p.modulePath,
			Pkg:       p.pkgName,
			Singleton: p.singleton,
			Version:   p.version,
		})
	}
	o.HasMQ = len(o.MQPlugins) > 0
	add(o.Registry)

	o.Components = comps
	o.UsedList = strings.Join(comps, ",")

	var pb strings.Builder
	for _, c := range comps {
		prefix := componentMeta[c].prefix
		if c == "sqlite" {
			prefix = o.ProjectName + ".db"
		}
		fmt.Fprintf(&pb, "      %s: %s\n", c, prefix)
	}
	o.PrefixBlock = pb.String()

	// 数据层: 优先 mysql > postgres > clickhouse
	o.DataLayer = "memory"
	o.DaoType = ""
	for _, c := range []string{"mysql", "postgres", "clickhouse"} {
		if seen[c] {
			o.DataLayer = c
			o.DaoType = map[string]string{
				"mysql":      "MySQLDao",
				"postgres":   "PostgresDao",
				"clickhouse": "ClickhouseDao",
			}[c]
			break
		}
	}

	// 配置中心 / server_type
	if o.ConfigCenter == "" || o.ConfigCenter == "none" {
		o.ServerType = "file"
		o.ConfigServer = ""
	} else {
		o.ServerType = o.ConfigCenter
		o.ConfigServer = map[string]string{
			"nacos":        "http://127.0.0.1:8848/",
			"consul":       "http://127.0.0.1:8500/",
			"etcd":         "127.0.0.1:2379",
			"polaris":      "http://127.0.0.1:8091/",
			"springconfig": "http://127.0.0.1:8888/",
		}[o.ConfigCenter]
		if o.ConfigCenter == "polaris" {
			o.ConfigToken = "" // 需用户自行填写
		}
	}
}

// buildAppYAML 构造主配置文件 application.yml。
//
// v2-arch 适配:
//   - 在 go.application.* 之外新增 go.framework.* 节点, 控制 v2 新增能力开关
//     (health 探针、metrics 端点、graceful shutdown 超时、LB 策略)。
//   - go.runtime.* 节点由 Makefile 在编译期通过 -ldflags 注入, 故此 YAML 里只
//     留注释说明; 不显式设置, 避免与编译期冲突。
//   - 旧键 (go.application.* / go.config.* / go.discovery.*) 100% 向后兼容。
func buildAppYAML(o *ProjectOptions) string {
	var b strings.Builder
	b.WriteString("go:\n")
	b.WriteString("  application:\n")
	fmt.Fprintf(&b, "    name: %s\n", o.ProjectName)
	fmt.Fprintf(&b, "    port: %d\n", o.Port)
	fmt.Fprintf(&b, "    project: %s\n", o.ProjectName)
	b.WriteString("    debug: true\n")
	b.WriteString("  logger:\n")
	b.WriteString("    level: debug\n")
	b.WriteString("    out: console,file\n")
	fmt.Fprintf(&b, "    file: logs/%s\n", o.ProjectName)
	b.WriteString("  config:\n")
	fmt.Fprintf(&b, "    used: \"%s\"\n", o.UsedList)
	fmt.Fprintf(&b, "    env: %s\n", o.Env)
	fmt.Fprintf(&b, "    server_type: %s\n", o.ServerType)
	if o.ServerType == "file" {
		b.WriteString("    path: conf\n")
	}
	if o.ConfigServer != "" {
		fmt.Fprintf(&b, "    server: %s\n", o.ConfigServer)
	}
	if o.ConfigToken != "" {
		fmt.Fprintf(&b, "    token: %s\n", o.ConfigToken)
	}
	b.WriteString("    prefix:\n")
	b.WriteString(o.PrefixBlock)

	// v2 框架元配置 (新增, 与 application.* 解耦; 旧项目未声明时按零值处理)。
	b.WriteString("  framework:\n")
	fmt.Fprintf(&b, "    health: %s    # K8s 探针: 启用 /health/live /health/ready /health/startup\n", boolYAML(o.Health))
	fmt.Fprintf(&b, "    metrics: %s   # Prometheus: 启用 /metrics 端点 + HTTP 指标埋点\n", boolYAML(o.Metrics))
	fmt.Fprintf(&b, "    otel: %s      # OpenTelemetry: 需业务侧 SetTracerProvider 后才真正生效\n", boolYAML(o.Otel))
	b.WriteString("    shutdownTimeout: 15  # 优雅关闭超时 (秒); HTTPS 与 HTTP 同时受控\n")
	b.WriteString("    loadBalancer: " + o.LBPolicy + "    # round/random/least/consistent (默认 round)\n")

	// v2 运行时元数据: 由 Makefile -ldflags 注入 (main.CommitHash / main.BuildTime
	// / main.Version / main.GoVersion), 运行时由 config.ConfigRuntime 自动填充,
	// YAML 里不要重复声明, 否则会被解析器覆盖编译期注入值。

	if o.Registry != "" && o.Registry != "none" {
		b.WriteString("  discovery:\n")
		fmt.Fprintf(&b, "    registry: %s\n", o.Registry)
		b.WriteString("    callType: json\n")
	}
	if o.JWT {
		b.WriteString("  jwt:\n")
		b.WriteString("    secret: 1234567890abcdef\n")
	}
	if o.Casbin {
		b.WriteString("  casbin:\n")
		b.WriteString("    enabled: true\n")
		b.WriteString("    model_file: casbin.conf\n")
	}
	if o.Sys {
		b.WriteString("  sys:\n")
		b.WriteString("    enabled: true\n")
		b.WriteString("    initdb: true\n")
		fmt.Fprintf(&b, "    baseUri: %s\n", o.BaseURI)
	}
	return b.String()
}

// boolYAML 把 bool 转 yaml 字面值 (true/false), 比 strconv.FormatBool 更直观。
func boolYAML(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// buildReadme 生成新工程的 README.md。
//
// v2-arch 适配: 新增 v2 能力小节 (health/metrics/otel/loadbalancer) 与
// go.framework.* 配置说明, 让业务侧一眼看清新版本能做什么。
func buildReadme(o *ProjectOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", o.ProjectName)
	b.WriteString("基于 [MGin v2](https://github.com/maczh/mgin/v2) 微服务框架生成的项目骨架。\n\n")

	b.WriteString("## 目录结构\n\n")
	b.WriteString("```\n")
	b.WriteString("main.go            程序入口 (含 health/metrics 挂载与 health.MarkStarted)\n")
	b.WriteString("version.go         版本号/编译时间/GitHash (由 -ldflags 注入, 不代码赋值)\n")
	b.WriteString("Makefile           本地构建脚本 (build/linux/build-multi-os/test/lint/cover)\n")
	b.WriteString("plugins.go         MQ 插件注册 (启用 MQ 时才有)\n")
	b.WriteString("conf/              配置文件 (application.yml + 各组件 <prefix>-<env>.yml)\n")
	b.WriteString("model/             数据模型 (GORM)\n")
	if o.DataLayer != "memory" {
		fmt.Fprintf(&b, "dao/               数据访问 (泛型 DAO: %s)\n", o.DaoType)
	}
	b.WriteString("service/           业务逻辑 (方法签名接收 context.Context)\n")
	b.WriteString("controller/        HTTP 处理函数 (用 errcode.Definition + i18n.ErrorDef 三向映射)\n")
	b.WriteString("router/            路由与中间件注册\n")
	b.WriteString("```\n\n")

	b.WriteString("## v2 新能力 (本工程开启情况)\n\n")
	fmt.Fprintf(&b, "| 能力 | 启用 | 说明 |\n")
	b.WriteString("|------|------|------|\n")
	fmt.Fprintf(&b, "| 健康检查 `/health/{live,ready,startup}` | %s | K8s 探针; `/health/ready` 按数据源 Check 自报健康 |\n", onOff(o.Health))
	fmt.Fprintf(&b, "| Prometheus `/metrics` | %s | HTTP 请求计数/直方图/in_flight + go_* 运行时 |\n\n", onOff(o.Metrics))
	fmt.Fprintf(&b, "| OpenTelemetry | %s | 业务侧 `otel.SetTracerProvider()` 启用; W3C traceparent 始终透传 |\n", onOff(o.Otel))
	fmt.Fprintf(&b, "| 客户端负载均衡 | %s | 策略: %s; 4 实例全熔断时返 ErrAllInstancesCircuitOpen |\n", onOff(o.LBPolicy != "" && o.Registry != "none" && o.Registry != "" && o.Registry != "none"), o.LBPolicy)

	b.WriteString("\n## 生成时选择的配置\n\n")
	fmt.Fprintf(&b, "- 模块路径: `%s`\n", o.Module)
	fmt.Fprintf(&b, "- 端口: `%d`\n", o.Port)
	fmt.Fprintf(&b, "- 数据库 (used): `%s`\n", o.UsedList)
	mq := strings.Join(o.MQ, ",")
	if mq == "" {
		mq = "none"
	}
	fmt.Fprintf(&b, "- 消息队列: `%s`\n", mq)
	fmt.Fprintf(&b, "- 注册中心: `%s`\n", o.Registry)
	fmt.Fprintf(&b, "- 配置中心: `%s`\n", o.ConfigCenter)
	fmt.Fprintf(&b, "- JWT: `%v`  Casbin: `%v`  i18n: `%v`\n", o.JWT, o.Casbin, o.I18n)

	b.WriteString("\n## 本地开发\n\n")
	b.WriteString("```bash\n")
	b.WriteString("make tidy      # 下载依赖 (走 https://goproxy.cn)\n")
	b.WriteString("make build     # 本地构建 (注入 Version/BuildTime/GitHash)\n")
	b.WriteString("make linux     # 交叉编译 Linux amd64 + upx 压缩\n")
	b.WriteString("make build-multi-os   # 同时出 linux/darwin/windows x amd64+arm64\n")
	b.WriteString("make run       # 本地直接 go run\n")
	b.WriteString("make test      # 跑全部单元测试\n")
	b.WriteString("make test-race # 跑 race detector\n")
	b.WriteString("make cover     # 输出 coverage.html\n")
	b.WriteString("make lint      # go vet + golangci-lint (可选)\n")
	b.WriteString("```\n\n")

	b.WriteString("## 运行\n\n")
	b.WriteString("```bash\n")
	fmt.Fprintf(&b, "./%s                    # 默认读 conf/application.yml\n", o.ProjectName)
	fmt.Fprintf(&b, "./%s -f conf/application.yml\n", o.ProjectName)
	fmt.Fprintf(&b, "./%s -v                  # 打印版本 (含 CommitHash/BuildTime)\n", o.ProjectName)
	if o.Health {
		b.WriteString("curl http://localhost:" + intToStr(o.Port) + "/health/live    # K8s liveness\n")
		b.WriteString("curl http://localhost:" + intToStr(o.Port) + "/health/ready   # K8s readiness\n")
	}
	if o.Metrics {
		b.WriteString("curl http://localhost:" + intToStr(o.Port) + "/metrics       # Prometheus 抓取\n")
	}
	b.WriteString("```\n\n")

	b.WriteString("> 提示: 启用数据库/消息队列/注册中心等组件后, 需先启动对应中间件, 否则框架会在启动时打印连接错误 (不影响进程启动)。\n\n")
	b.WriteString("> 迁移指南: 旧 v1 工程升级到 v2 请参考 [mgin-v2 迁移文档](https://github.com/maczh/mgin/v2/blob/v2-arch/docs/migration-v1-to-v2.md)。\n")
	return b.String()
}

// onOff 把 bool 转成"已开启/未开启", 给 README 表格用。
func onOff(v bool) string {
	if v {
		return "✅ 已开启"
	}
	return "❌ 未开启"
}

// intToStr 避免 import strconv 增加 dependency, 实现简单 int 转 string。
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// render 解析并执行一个 text/template。
func render(tmpl string, data interface{}) string {
	t, err := template.New("t").Parse(tmpl)
	if err != nil {
		// 模板为内置常量, 正常不会触发
		return "// template error: " + err.Error()
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "// execute error: " + err.Error()
	}
	return buf.String()
}
