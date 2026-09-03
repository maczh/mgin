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

// buildReadme 生成新工程的 README.md。
func buildReadme(o *ProjectOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", o.ProjectName)
	b.WriteString("基于 [MGin](https://github.com/maczh/mgin) 微服务框架生成的项目骨架。\n\n")
	b.WriteString("## 目录结构\n\n")
	b.WriteString("```\n")
	b.WriteString("main.go            程序入口\n")
	b.WriteString("conf/              配置文件 (application.yml + 各组件 <prefix>-<env>.yml)\n")
	b.WriteString("model/             数据模型 (GORM)\n")
	if o.DataLayer != "memory" {
		b.WriteString("dao/               数据访问 (泛型 DAO)\n")
	}
	b.WriteString("service/           业务逻辑\n")
	b.WriteString("controller/        HTTP 处理函数\n")
	b.WriteString("router/            路由与中间件注册\n")
	b.WriteString("```\n\n")
	b.WriteString("## 生成时选择的配置\n\n")
	fmt.Fprintf(&b, "- 模块路径: `%s`\n", o.Module)
	fmt.Fprintf(&b, "- 端口: `%d`\n", o.Port)
	fmt.Fprintf(&b, "- 数据库(used): `%s`\n", o.UsedList)
	mq := strings.Join(o.MQ, ",")
	if mq == "" {
		mq = "none"
	}
	fmt.Fprintf(&b, "- 消息队列: `%s`\n", mq)
	fmt.Fprintf(&b, "- 注册中心: `%s`\n", o.Registry)
	fmt.Fprintf(&b, "- 配置中心: `%s`\n", o.ConfigCenter)
	fmt.Fprintf(&b, "- JWT: `%v`  Casbin: `%v`  i18n: `%v`\n", o.JWT, o.Casbin, o.I18n)
	b.WriteString("\n## 运行\n\n")
	b.WriteString("```bash\n")
	b.WriteString("go mod tidy\n")
	fmt.Fprintf(&b, "go build -o %s .\n", o.ProjectName)
	fmt.Fprintf(&b, "./%s              # 读取 conf/application.yml\n", o.ProjectName)
	fmt.Fprintf(&b, "./%s -f conf/application.yml\n", o.ProjectName)
	fmt.Fprintf(&b, "./%s -v           # 查看版本\n", o.ProjectName)
	b.WriteString("```\n\n")
	b.WriteString("> 提示: 启用数据库/消息队列/注册中心等组件后, 需先启动对应中间件, 否则框架会在启动时打印连接错误(不影响进程启动)。\n")
	return b.String()
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
