package i18n

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type sqlRecorder struct {
	logger.Interface
	statements []string
}

func (r *sqlRecorder) Trace(_ context.Context, _ time.Time, sql func() (string, int64), _ error) {
	statement, _ := sql()
	r.statements = append(r.statements, statement)
}

// Verify the real MySQL dialect without opening a network connection.
func TestMySQLDictionarySQL(t *testing.T) {
	recorder := &sqlRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	conn, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "test:test@tcp(127.0.0.1:3306)/test?parseTime=true",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true, Logger: recorder})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s, err := NewStore(conn, "demo.app-test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	entry := &Entry{MessageID: "hello", Language: "en", Text: "Hello"}
	if err := s.Create(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "hello", "en"); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(ctx, entry); err != nil {
		t.Fatal(err)
	}
	_ = s.Delete(ctx, "hello", "en") // Dry-run rows affected is zero.
	_, _, err = s.List(ctx, ListOptions{Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if len(recorder.statements) < 7 {
		t.Fatalf("statements=%v", recorder.statements)
	}
	for _, sql := range recorder.statements {
		if !strings.Contains(sql, "`demo.app-test_i18n`") {
			t.Errorf("table identifier not quoted as one name: %s", sql)
		}
	}
	ddl := recorder.statements[0]
	if !strings.Contains(ddl, "PRIMARY KEY (message_id, language)") || !strings.Contains(ddl, "utf8mb4_bin") {
		t.Fatalf("DDL=%s", ddl)
	}
	if !strings.HasPrefix(recorder.statements[1], "INSERT INTO `demo.app-test_i18n`") {
		t.Fatal(recorder.statements[1])
	}
}
