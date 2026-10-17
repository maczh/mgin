package i18n

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/db"
	mysqlclient "github.com/maczh/mgin/db/mysql"
	sqliteclient "github.com/maczh/mgin/db/sqlite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "dictionary.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	s, err := NewStore(conn, "demo.app-test")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDictionaryCRUDAndFallback(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if s.TableName() != "demo.app-test_i18n" {
		t.Fatal(s.TableName())
	}
	entries := []Entry{
		{MessageID: "hello", Language: "zh_CN", Text: "你好"},
		{MessageID: "hello", Language: "EN", Text: "Hello"},
		{MessageID: "hello", Language: "fr", Text: "Bonjour"},
		{MessageID: "name:' OR 1=1 --", Language: "en", Text: "literal key"},
	}
	for i := range entries {
		if err := s.Create(ctx, &entries[i]); err != nil {
			t.Fatal(err)
		}
	}
	if entries[0].Language != "zh-cn" || entries[0].CreatedAt.IsZero() {
		t.Fatalf("entry=%+v", entries[0])
	}
	if err := s.Create(ctx, &Entry{MessageID: "hello", Language: "en", Text: "duplicate"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate=%v", err)
	}
	for _, test := range []struct{ key, lang, want string }{
		{"hello", "en", "Hello"}, {"hello", "de", "你好"}, {"hello", "", "你好"}, {"name:' OR 1=1 --", "en", "literal key"},
	} {
		got, err := s.Translate(ctx, test.key, test.lang)
		if err != nil || got != test.want {
			t.Fatalf("translate(%q,%q)=%q,%v", test.key, test.lang, got, err)
		}
	}
	if _, err := s.Translate(ctx, "missing", "en"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	update := &Entry{MessageID: "hello", Language: "EN", Text: "Updated"}
	if err := s.Update(ctx, update); err != nil {
		t.Fatal(err)
	}
	if text, err := s.Translate(ctx, "hello", "en"); err != nil || text != "Updated" {
		t.Fatalf("updated=%q,%v", text, err)
	}
	list, total, err := s.List(ctx, ListOptions{Language: "EN", Page: 1, PageSize: 1})
	if err != nil || len(list) != 1 || total != 2 {
		t.Fatalf("list=%v,%d,%v", list, total, err)
	}
	if err := s.Delete(ctx, "hello", "en"); err != nil {
		t.Fatal(err)
	}
	if text, err := s.Translate(ctx, "hello", "en"); err != nil || text != "你好" {
		t.Fatalf("deleted fallback=%q,%v", text, err)
	}
	if err := s.Delete(ctx, "hello", "en"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.Update(ctx, &Entry{MessageID: "missing", Language: "en", Text: "no"}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestDictionaryValidationAndCancellation(t *testing.T) {
	s := testStore(t)
	for _, entry := range []*Entry{
		nil, {}, {MessageID: "hello", Language: "en; DROP TABLE test", Text: "bad"},
		{MessageID: strings.Repeat("a", 192), Language: "en", Text: "bad"},
		{MessageID: "hello", Language: "en", Text: strings.Repeat("a", 65536)},
	} {
		if err := s.Create(context.Background(), entry); !errors.Is(err, ErrInvalidEntry) {
			t.Fatalf("invalid=%v", err)
		}
	}
	if _, err := NewStore(s.conn, "bad;name"); err == nil {
		t.Fatal("unsafe table name accepted")
	}
	if _, _, err := s.List(context.Background(), ListOptions{Page: 1000001}); !errors.Is(err, ErrInvalidEntry) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := s.List(ctx, ListOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
}

func TestDictionaryConcurrentDuplicate(t *testing.T) {
	s := testStore(t)
	var wg sync.WaitGroup
	successes := make(chan struct{}, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.Create(context.Background(), &Entry{MessageID: "one", Language: "en", Text: "One"})
			if err == nil {
				successes <- struct{}{}
			} else if !errors.Is(err, ErrConflict) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(successes) != 1 {
		t.Fatalf("created %d duplicates", len(successes))
	}
}

func testConfig(t *testing.T, used string) string {
	t.Helper()
	Close()
	old := *config.Config
	t.Cleanup(func() { Close(); *config.Config = old })
	dir := t.TempDir()
	name := filepath.Join(dir, "application.yml")
	text := "go:\n  application:\n    name: local-app\n  config:\n    used: \"" + used + "\"\n  i18n:\n    default: en\n"
	if err := os.WriteFile(name, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	config.Config.Init(name)
	return dir
}

func TestSQLiteFallbackPersistence(t *testing.T) {
	dir := testConfig(t, "")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	s, err := GetStore()
	if err != nil {
		t.Fatal(err)
	}
	if !s.owned || s.TableName() != "local-app_i18n" {
		t.Fatalf("store=%+v", s)
	}
	if err := s.Create(context.Background(), &Entry{MessageID: "welcome", Language: "en", Text: "Welcome {}"}); err != nil {
		t.Fatal(err)
	}
	if got := String("welcome"); got != "Welcome {}" {
		t.Fatal(got)
	}
	if got := Format("welcome", "Sam"); got != "Welcome Sam" {
		t.Fatal(got)
	}
	if got := String("missing"); got != "missing" {
		t.Fatal(got)
	}
	if _, err := os.Stat(filepath.Join(dir, "local-app_i18n.db")); err != nil {
		t.Fatal(err)
	}
	Close()
	if _, err := GetStore(); !errors.Is(err, ErrNotInitialized) {
		t.Fatal(err)
	}
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	if got := GetString("welcome", "de"); got != "Welcome {}" {
		t.Fatalf("persisted=%q", got)
	}
}

func TestConfiguredMySQLDoesNotSilentlyFallback(t *testing.T) {
	dir := testConfig(t, "mysql")
	old := db.Mysql
	db.Mysql = &mysqlclient.MysqlClient{}
	defer func() { db.Mysql = old }()
	if err := Init(); err == nil {
		t.Fatal("uninitialized MySQL was accepted")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.db"))
	if err != nil || len(files) != 0 {
		t.Fatalf("unexpected SQLite fallback: %v,%v", files, err)
	}
}

func TestSharedSQLiteConnectionRemainsOpen(t *testing.T) {
	dir := testConfig(t, "sqlite")
	old := db.Sqlite
	db.Sqlite = &sqliteclient.Sqlite{}
	defer func() { db.Sqlite.Close(); db.Sqlite = old }()
	db.Sqlite.Init(filepath.Join(dir, "shared.db"))
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	s, err := GetStore()
	if err != nil {
		t.Fatal(err)
	}
	if s.owned {
		t.Fatal("shared connection marked owned")
	}
	Close()
	conn, err := db.Sqlite.GetConnection()
	if err != nil {
		t.Fatal(err)
	}
	pool, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(); err != nil {
		t.Fatal("shared connection was closed", err)
	}
}
