package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/knadh/koanf"
	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/db"
	mysqlclient "github.com/maczh/mgin/v2/pkg/db/mysql"
)

func TestMysqlPluginPropagatesInitError(t *testing.T) {
	oldConfig := *config.Config
	oldMysql := db.Mysql
	t.Cleanup(func() {
		*config.Config = oldConfig
		db.Mysql = oldMysql
	})

	config.Config.Cnf = koanf.New(".")
	config.Config.WorkDir = t.TempDir()
	config.Config.Config.Type = "file"
	config.Config.Config.Env = "test"
	config.Config.Config.Used = "mysql"
	config.Config.Config.Prefix.Mysql = "mysql"

	data := []byte("go:\n  data:\n    mysql: user:pass@tcp(127.0.0.1:1)/mgin?timeout=100ms\n")
	if err := os.WriteFile(filepath.Join(config.Config.WorkDir, "mysql-test.yml"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	db.Mysql = &mysqlclient.MysqlClient{}
	p := Mysql()
	if !p.Enabled() {
		t.Fatal("mysql plugin is not enabled")
	}
	if err := p.Init(context.Background()); err == nil {
		t.Fatal("expected MySQL initialization error")
	}
}
