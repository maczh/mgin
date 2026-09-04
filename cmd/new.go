package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// 交互式问答使用的可选项列表。
var (
	dbOptions = []string{"mysql", "postgres", "sqlite", "mongodb", "redis", "clickhouse", "elasticsearch"}
	// 消息队列支持多选; 直接回车(空)表示不使用任何消息队列
	mqOptions           = []string{"none", "nats", "kafka", "mqtt", "rabbit"}
	registryOptions     = []string{"none", "nacos", "consul", "etcd"}
	configCenterOptions = []string{"none", "file", "nacos", "consul", "etcd", "polaris", "springconfig"}
)

// runNew 解析 `mgin new` 子命令的参数, 必要时通过 CLI 菜单问答收集配置, 然后生成工程骨架。
func runNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	var (
		module       string
		port         int
		dbs          string
		mq           string
		registry     string
		configCenter string
		i18n         bool
		jwt          bool
		casbin       bool
		sys          bool
		output       string
		force        bool
		interactive  bool
		mginVersion  string
	)
	fs.StringVar(&module, "module", "", "Go module 路径")
	fs.IntVar(&port, "port", 8080, "HTTP 端口")
	fs.StringVar(&dbs, "db", "", "数据库列表, 逗号分隔")
	fs.StringVar(&mq, "mq", "", "消息队列, 逗号分隔多选: nats/kafka/mqtt/rabbit/none")
	fs.StringVar(&registry, "registry", "", "注册中心: nacos/consul/etcd/none")
	fs.StringVar(&configCenter, "config-center", "", "配置中心: nacos/consul/etcd/polaris/springconfig/file/none")
	fs.BoolVar(&i18n, "i18n", false, "启用国际化")
	fs.BoolVar(&jwt, "jwt", false, "启用 JWT 鉴权")
	fs.BoolVar(&casbin, "casbin", false, "启用 Casbin 接口鉴权")
	fs.BoolVar(&sys, "sys", false, "启用内置系统管理模块")
	fs.StringVar(&output, "output", "", "输出目录")
	fs.BoolVar(&force, "force", false, "覆盖已存在目录")
	fs.StringVar(&mginVersion, "mgin-version", "", "指定 mgin 依赖版本 (默认自动获取最新发布版)")
	fs.BoolVar(&interactive, "interactive", false, "强制使用交互式问答模式")
	fs.BoolVar(&interactive, "i", false, "强制使用交互式问答模式(简写)")

	// v2 新能力开关: --health / --metrics / --otel / --loadbalancer
	var (
		healthFlag   bool
		metricsFlag  bool
		otelFlag     bool
		loadBalancer string
	)
	fs.BoolVar(&healthFlag, "health", false, "v2: 启用 /health/{live,ready,startup} 探针 (go.framework.engine)")
	fs.BoolVar(&metricsFlag, "metrics", false, "v2: 启用 /metrics 端点 (Prometheus 指标)")
	fs.BoolVar(&otelFlag, "otel", false, "v2: 启用 OpenTelemetry (业务侧需自接 TracerProvider)")
	fs.StringVar(&loadBalancer, "loadbalancer", "", "v2: 客户端负载均衡策略 round/random/least/consistent (默认 round)")

	// Go 的 flag 包在遇到第一个非 flag 位置参数时会停止解析, 导致后续 flag 被忽略。
	// 这里手动切分: 识别出「取值型 flag」(如 -module / -output) 并连同其后的取值一起保留,
	// 其余非 flag 的 token 视为位置参数(工程名取第一个)。这样无论 flag 写在工程名前还是后都能正确解析。
	positionals, flagArgs, err := splitArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	projectName := ""
	if len(positionals) > 0 {
		projectName = strings.TrimSpace(positionals[0])
	}

	opts := &ProjectOptions{
		ProjectName:  projectName,
		Port:         port,
		I18n:         i18n,
		JWT:          jwt,
		Casbin:       casbin,
		Sys:          sys,
		OutputDir:    output,
		Force:        force,
		MginVersion:  mginVersion,
		Module:       module,
		MQ:           normalizeMQs(mq),
		Registry:     normalizeChoice(registry, "none", registryOptions),
		ConfigCenter: normalizeChoice(configCenter, "none", configCenterOptions),
		DBs:          normalizeDBs(dbs),
		Health:       healthFlag,
		Metrics:      metricsFlag,
		Otel:         otelFlag,
		LBPolicy:     normalizeChoice(loadBalancer, "", lbStrategies),
	}

	// 交互式条件: 显式 -i, 或检测到终端(TTY)。
	// 当通过管道传入数据(如 CI / 自动化)时 isTerminal 为 false, 走下面的默认值补齐逻辑。
	forceInteractive := interactive || isTerminal(os.Stdin)
	if projectName == "" && !forceInteractive {
		return fmt.Errorf("请指定工程名, 例如: mgin new myservice  或  mgin new -i")
	}

	if forceInteractive {
		reader := bufio.NewReader(os.Stdin)
		if err := runInteractive(reader, opts); err != nil {
			return err
		}
	}

	// 非交互模式下, 对仍未指定的项补齐默认值。
	if opts.Module == "" {
		opts.Module = "github.com/maczh/" + opts.ProjectName
	}
	if opts.Port == 0 {
		opts.Port = 8080
	}
	if len(opts.DBs) == 0 {
		opts.DBs = []string{"mysql"}
	}
	// 负载均衡策略兜底: 未指定时给 round (v2 默认)。
	if opts.LBPolicy == "" {
		opts.LBPolicy = "round"
	}

	// 解析 mgin 依赖版本: 显式 --mgin-version 优先; 否则自动获取最新发布版(离线回退默认版本)。
	if opts.MginVersion == "" {
		opts.MginVersion = resolveMginVersion()
	}
	if opts.MginVersion == "" {
		opts.MginVersion = defaultMginVersion
	}

	return scaffold(opts)
}

// defaultMginVersion 是离线或拉取失败时的回退版本, 保证脚手架始终可用。
// v2-arch 说明: 本分支是对 v1.25 系列的重构 (v2.0 骨架 + v2.1 微服务能力),
// 使用 /v2 模块路径和 v2 版本命名空间。
const defaultMginVersion = "v2.0.0"

// mginModulePath 是脚手架生成的工程所依赖的 mgin 模块路径。
const mginModulePath = "github.com/maczh/mgin/v2"

// resolveMginVersion 通过 Go 模块代理获取 mgin 的最新发布版本号。
// 优先使用 GOPROXY 环境变量中的第一个代理地址(忽略 direct/off),
// 拉取失败(无网络等)时回退到 defaultMginVersion。
func resolveMginVersion() string {
	proxy := os.Getenv("GOPROXY")
	base := ""
	for _, p := range strings.Split(proxy, ",") {
		p = strings.TrimSpace(p)
		if p != "" && p != "direct" && p != "off" {
			base = p
			break
		}
	}
	if base == "" {
		base = "https://goproxy.cn"
	}
	base = strings.TrimRight(base, "/")

	url := base + "/" + mginModulePath + "/@v/list"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return defaultMginVersion
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return defaultMginVersion
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return defaultMginVersion
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return defaultMginVersion
	}

	best := ""
	for _, line := range strings.Split(string(body), "\n") {
		v := strings.TrimSpace(line)
		if v == "" || !semver.IsValid(v) {
			continue
		}
		if best == "" || semver.Compare(v, best) > 0 {
			best = v
		}
	}
	if best == "" {
		return defaultMginVersion
	}
	return best
}

// runInteractive 通过菜单式问答收集全部工程配置。
// 已通过 flag 提供的取值会作为默认值展示, 用户直接回车即可沿用。
func runInteractive(r *bufio.Reader, opts *ProjectOptions) error {
	if opts.ProjectName == "" {
		opts.ProjectName = "myservice"
	}
	opts.ProjectName = askText(r, "工程名 (Project Name)", opts.ProjectName)
	if opts.Module == "" {
		opts.Module = "github.com/maczh/" + opts.ProjectName
	}
	opts.Module = askText(r, "Go module 路径", opts.Module)
	opts.Port = askInt(r, "HTTP 端口 (Port)", opts.Port, 8080)
	opts.DBs = askMulti(r, "数据库 (可多选, 输入序号, 逗号分隔, 如 1,3)", dbOptions, opts.DBs)
	opts.MQ = askMulti(r, "消息队列 (可多选, 输入序号, 逗号分隔; 选 1 或直接回车表示不使用)", mqOptions, opts.MQ)
	opts.Registry = singleFrom(r, "注册中心 (Registry)", registryOptions, opts.Registry)
	opts.ConfigCenter = singleFrom(r, "配置中心 (Config Center)", configCenterOptions, opts.ConfigCenter)
	opts.JWT = askBool(r, "启用 JWT 鉴权", opts.JWT)
	opts.Casbin = askBool(r, "启用 Casbin 接口级鉴权", opts.Casbin)
	opts.I18n = askBool(r, "启用国际化 (i18n, 需 xlang 服务)", opts.I18n)
	opts.Sys = askBool(r, "启用内置系统管理模块", opts.Sys)
	// v2 新能力交互问答
	opts.Health = askBool(r, "v2: 启用 /health 探针 (K8s liveness/readiness/startup)", opts.Health)
	opts.Metrics = askBool(r, "v2: 启用 /metrics 端点 (Prometheus 指标)", opts.Metrics)
	opts.Otel = askBool(r, "v2: 启用 OpenTelemetry (业务侧需自接 TracerProvider)", opts.Otel)
	if opts.Registry != "none" {
		opts.LBPolicy = singleFrom(r, "v2: 客户端负载均衡策略", []string{"round", "random", "least", "consistent"}, opts.LBPolicy)
	} else if opts.LBPolicy == "" {
		opts.LBPolicy = "round"
	}
	return nil
}

// askText 读取一行文本, 为空时返回默认值 def。
func askText(r *bufio.Reader, label, def string) string {
	fmt.Printf("%s [%s]: ", label, def)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

// askInt 读取一个正整数, 非法或 <=0 时回退到 fallback。
func askInt(r *bufio.Reader, label string, def, fallback int) int {
	s := askText(r, label, strconv.Itoa(def))
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// indexOf 返回 s 在 list 中的下标, 不存在返回 -1。
func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// singleFrom 展示单选菜单, 返回用户选择的选项字符串。
func singleFrom(r *bufio.Reader, label string, options []string, def string) string {
	defIdx := indexOf(options, def)
	if defIdx < 0 {
		defIdx = 0
	}
	fmt.Printf("%s\n", label)
	for i, o := range options {
		mark := "  "
		if i == defIdx {
			mark = "* "
		}
		fmt.Printf("  %d) %s%s\n", i+1, mark, o)
	}
	for {
		fmt.Printf("请选择 [1-%d, 默认 %d]: ", len(options), defIdx+1)
		line, err := r.ReadString('\n')
		if err != nil && line == "" {
			return options[defIdx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return options[defIdx]
		}
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > len(options) {
			fmt.Println("  输入无效, 请重新选择")
			continue
		}
		return options[n-1]
	}
}

// askBool 以「否/是」菜单读取布尔值。
func askBool(r *bufio.Reader, label string, def bool) bool {
	opts := []string{"否", "是"}
	defIdx := 0
	if def {
		defIdx = 1
	}
	return singleFrom(r, label, opts, opts[defIdx]) == "是"
}

// askMulti 展示多选菜单, 返回用户选择的选项切片(基于序号, 逗号分隔)。
func askMulti(r *bufio.Reader, label string, options []string, current []string) []string {
	fmt.Printf("%s\n", label)
	for i, o := range options {
		mark := "  "
		if indexOf(current, o) >= 0 {
			mark = "* "
		}
		fmt.Printf("  %d) %s%s\n", i+1, mark, o)
	}
	fmt.Printf("请选择 [序号逗号分隔, 默认当前选择]: ")
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return current
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return current
	}
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > len(options) {
			continue
		}
		o := options[n-1]
		if !seen[o] {
			seen[o] = true
			out = append(out, o)
		}
	}
	if len(out) == 0 {
		return current
	}
	return out
}

// normalizeDBs 解析并过滤数据库列表。
func normalizeDBs(s string) []string {
	allowed := map[string]bool{
		"mysql": true, "postgres": true, "sqlite": true, "mongodb": true,
		"redis": true, "clickhouse": true, "elasticsearch": true,
	}
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		p := strings.ToLower(strings.TrimSpace(part))
		if p == "" || !allowed[p] || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	if len(out) == 0 {
		out = []string{"mysql"}
	}
	return out
}

// normalizeMQs 解析消息队列多选值(逗号分隔), 支持 nats/kafka/mqtt/rabbit。
// "none" 与空值表示不使用消息队列; 非法值被忽略; 结果按 mqOrder 排序, 保证输出稳定。
func normalizeMQs(s string) []string {
	allowed := map[string]bool{"nats": true, "kafka": true, "mqtt": true, "rabbit": true}
	picked := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		p := strings.ToLower(strings.TrimSpace(part))
		if p == "" || p == "none" || !allowed[p] {
			continue
		}
		picked[p] = true
	}
	var out []string
	for _, key := range mqOrder {
		if picked[key] {
			out = append(out, key)
		}
	}
	return out
}

// normalizeChoice 规范化单选，非法值回退到 def。
func normalizeChoice(s, def string, allowed []string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return def
	}
	for _, a := range allowed {
		if s == a {
			return s
		}
	}
	return def
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// isTerminal 判断是否为交互式终端。
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// boolFlags / valueFlags 列出 new 子命令的布尔型与取值型 flag 名称(同时支持 -x 与 --x 写法)。
var (
	boolFlags = map[string]bool{
		"-i": true, "-interactive": true, "-i18n": true, "-jwt": true,
		"-casbin": true, "-sys": true, "-force": true,
	}
	valueFlags = map[string]bool{
		"-module": true, "-port": true, "-db": true, "-mq": true,
		"-registry": true, "-config-center": true, "-output": true,
		"-mgin-version": true, "-loadbalancer": true,
	}
)

// splitArgs 将参数切分为位置参数(工程名)与 flag 参数。
// 取值型 flag 会连同其后的取值一并保留, 避免位置参数抢占 flag 的取值。
func splitArgs(args []string) (positionals, flagArgs []string, err error) {
	norm := func(s string) string {
		if strings.HasPrefix(s, "--") {
			return "-" + s[2:]
		}
		return s
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			positionals = append(positionals, a)
			continue
		}
		// -name=value 形式
		if eq := strings.Index(a, "="); eq >= 0 {
			flagArgs = append(flagArgs, a)
			continue
		}
		if valueFlags[norm(a)] {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("flag %s 需要一个参数", a)
			}
			if strings.HasPrefix(args[i+1], "-") {
				return nil, nil, fmt.Errorf("flag %s 需要一个参数", a)
			}
			flagArgs = append(flagArgs, a, args[i+1])
			i++
			continue
		}
		// 布尔型或未知 flag, 不带取值
		flagArgs = append(flagArgs, a)
	}
	return positionals, flagArgs, nil
}
