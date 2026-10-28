package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRegistries(t *testing.T) {
	registries, err := parseRegistries("jhris:nacos:nacos::jhris:DEFAULT,finance-bank:nacos:nacos:finance:bank_account:FINANCE")
	if err != nil {
		t.Fatal(err)
	}
	if len(registries) != 2 {
		t.Fatalf("got %d registries, want 2", len(registries))
	}
	finance := registries[1]
	if finance.Name != "finance-bank" || finance.Prefix != "nacos" || finance.Namespace != "finance" || finance.Group != "bank_account" || finance.ClusterName != "FINANCE" {
		t.Fatalf("unexpected registry: %#v", finance)
	}
}

func TestScaffoldWritesCustomRegistryPrefix(t *testing.T) {
	root := t.TempDir()
	opts := &ProjectOptions{
		ProjectName: "demo", Module: "example.com/demo", Port: 8080,
		DBs: []string{"redis"}, OutputDir: root, MginVersion: defaultMginVersion,
		NacosAPIVersion: "v2",
		Registries:      []RegistryOption{{Name: "finance", Type: "nacos", Prefix: "nacos-finance", Namespace: "finance"}},
	}
	if err := scaffold(opts); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "demo", "conf", "nacos-finance-test.yml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "apiversion: v2") {
		t.Fatalf("custom registry config has wrong version:\n%s", data)
	}
	goMod, err := os.ReadFile(filepath.Join(root, "demo", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goMod), "github.com/maczh/mgin v1.25.30-jh") {
		t.Fatalf("generated go.mod has wrong mgin version:\n%s", goMod)
	}
}

func TestParseRegistriesRejectsInvalidInput(t *testing.T) {
	for _, value := range []string{"missing-type", "a:unknown", "a:nacos,a:etcd"} {
		if _, err := parseRegistries(value); err == nil {
			t.Errorf("parseRegistries(%q) unexpectedly succeeded", value)
		}
	}
}

func TestBuildAppYAMLWithNamedRegistriesAndNacosV3(t *testing.T) {
	opts := &ProjectOptions{
		ProjectName: "gateway", Port: 8080, Env: "test", UsedList: "nacos",
		PrefixBlock: "      nacos: nacos\n", ServerType: "nacos",
		ConfigServer: "http://127.0.0.1:8848/", NacosAPIVersion: "v3",
		DefaultRegistry: "iot",
		Registries: []RegistryOption{
			{Name: "jhris", Type: "nacos", Prefix: "nacos", Group: "jhris", ClusterName: "DEFAULT"},
			{Name: "iot", Type: "nacos", Prefix: "nacos", Namespace: "iot-ns", Group: "iot"},
		},
	}
	yaml := buildAppYAML(opts)
	for _, want := range []string{
		"apiversion: v3", `defaultRegistry: "iot"`, `name: "jhris"`,
		`group: "jhris"`, `namespace: "iot-ns"`,
	} {
		if !strings.Contains(yaml, want) {
			t.Errorf("generated YAML does not contain %q:\n%s", want, yaml)
		}
	}
}

func TestDefaultMginVersion(t *testing.T) {
	if defaultMginVersion != "v1.25.30-jh" {
		t.Fatalf("defaultMginVersion = %q", defaultMginVersion)
	}
}

func TestNacosComponentTemplateUsesSelectedVersion(t *testing.T) {
	content := render(componentTemplates["nacos"], &ProjectOptions{NacosAPIVersion: "v3"})
	for _, want := range []string{"apiversion: v3", `username: ""`, `accessToken: ""`} {
		if !strings.Contains(content, want) {
			t.Errorf("Nacos config does not contain %q:\n%s", want, content)
		}
	}
}

func TestNormalizePluginsAndGenerateRegistration(t *testing.T) {
	opts := &ProjectOptions{
		DBs:     []string{"redis"},
		Plugins: normalizePlugins("s3,opensearch,postgres,s3,unknown"),
	}
	computeDerived(opts)
	if got, want := strings.Join(opts.Plugins, ","), "opensearch,postgres,s3"; got != want {
		t.Fatalf("plugins = %q, want %q", got, want)
	}
	if !opts.HasPlugins || len(opts.PluginDefs) != 3 {
		t.Fatalf("unexpected plugin metadata: %#v", opts.PluginDefs)
	}
	if opts.DataLayer != "memory" {
		t.Fatalf("external database plugin unexpectedly selected business DAO: %s", opts.DataLayer)
	}
	if opts.GoVersion != "1.25.9" {
		t.Fatalf("GoVersion = %q, want 1.25.9 for opensearch", opts.GoVersion)
	}
	generated := render(tmplPlugins, opts)
	for _, want := range []string{
		`"github.com/maczh/opensearch"`,
		`"github.com/maczh/postgres"`,
		`app.MGin.UsePlugin("opensearch", opensearch.OpenSearch)`,
		`app.MGin.UsePlugin("postgres", postgres.Postgres)`,
		`app.MGin.UsePlugin("s3", s3.NewS3())`,
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("plugins.go does not contain %q:\n%s", want, generated)
		}
	}
}

func TestAllPluginMetadataIsComplete(t *testing.T) {
	for _, key := range pluginOrder {
		plugin, ok := externalPlugins[key]
		if !ok || plugin.modulePath == "" || plugin.pkgName == "" || plugin.expr == "" || plugin.version == "" {
			t.Errorf("incomplete metadata for plugin %q: %#v", key, plugin)
		}
		if _, ok := componentTemplates[key]; !ok {
			t.Errorf("plugin %q has no configuration template", key)
		}
	}
}
