package i18n

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var (
	ErrNotInitialized = errors.New("i18n: dictionary is not initialized")
	ErrInvalidEntry   = errors.New("i18n: invalid dictionary entry")
	ErrNotFound       = errors.New("i18n: dictionary entry not found")
	ErrConflict       = errors.New("i18n: messageId and language already exist")
	tablePattern      = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,59}$`)
	languagePattern   = regexp.MustCompile(`^[a-z]{2,8}(-[a-z0-9]{1,8})*$`)
	currentStore      atomic.Pointer[Store]
	initMu            sync.Mutex
)

// Entry stores one translation. A message can have one entry per language.
type Entry struct {
	MessageID string    `json:"messageId" gorm:"column:message_id;size:191;primaryKey;autoIncrement:false"`
	Language  string    `json:"language" gorm:"size:35;primaryKey;autoIncrement:false"`
	Text      string    `json:"text" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store uses the application's relational connection, or owns a fallback SQLite connection.
type Store struct {
	conn            *gorm.DB
	table           string
	defaultLanguage string
	owned           bool
}

// NewStore creates the dictionary table on an existing GORM connection.
// The caller retains ownership of the connection.
func NewStore(conn *gorm.DB, appName string) (*Store, error) {
	if conn == nil {
		return nil, ErrNotInitialized
	}
	// Keep identifiers within MySQL’s 64-byte table-name limit.
	if !tablePattern.MatchString(appName) {
		return nil, fmt.Errorf("i18n: app name must contain 1–59 ASCII letters, digits, dots, underscores or hyphens")
	}
	local := conn.Session(&gorm.Session{NewDB: true})
	local.Config.TranslateError = true
	s := &Store{conn: local, table: appName + "_i18n", defaultLanguage: "zh-cn"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.createTable(ctx); err != nil {
		return nil, fmt.Errorf("i18n: creating dictionary table: %w", err)
	}
	return s, nil
}

func (s *Store) TableName() string { return s.table }
func (s *Store) query(ctx context.Context) *gorm.DB {
	q := s.conn.WithContext(ctx)
	q.Statement.Table = s.table
	q.Statement.TableExpr = &clause.Expr{SQL: s.quotedTable()}
	return q
}

// Quote the complete application table name as one identifier, including dots.
func (s *Store) quotedTable() string {
	if s.conn.Dialector.Name() == "mysql" {
		return "`" + s.table + "`"
	}
	return `"` + s.table + `"`
}

func (s *Store) createTable(ctx context.Context) error {
	timestampType := "TIMESTAMP"
	suffix := ""
	switch s.conn.Dialector.Name() {
	case "mysql":
		timestampType = "DATETIME(3)"
		suffix = " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin"
	case "sqlite":
		timestampType = "DATETIME"
	default:
		return errors.New("i18n: unsupported database driver")
	}
	// Only the validated, quoted table identifier and fixed dialect types are interpolated.
	query := "CREATE TABLE IF NOT EXISTS " + s.quotedTable() + ` (
 message_id VARCHAR(191) NOT NULL,
 language VARCHAR(35) NOT NULL,
 text TEXT NOT NULL,
 created_at ` + timestampType + ` NOT NULL,
 updated_at ` + timestampType + ` NOT NULL,
 PRIMARY KEY (message_id, language)
 )` + suffix
	return s.conn.WithContext(ctx).Exec(query).Error
}

func normalizeLanguage(language string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(language), "_", "-"))
}

func validateKey(messageID, language string) error {
	if !utf8.ValidString(messageID) || strings.TrimSpace(messageID) == "" || utf8.RuneCountInString(messageID) > 191 {
		return fmt.Errorf("%w: messageId must contain 1–191 characters", ErrInvalidEntry)
	}
	if len(language) > 35 || !languagePattern.MatchString(language) {
		return fmt.Errorf("%w: language must be a language tag of at most 35 characters", ErrInvalidEntry)
	}
	return nil
}

func validateEntry(entry *Entry) error {
	if entry == nil {
		return ErrInvalidEntry
	}
	entry.Language = normalizeLanguage(entry.Language)
	if err := validateKey(entry.MessageID, entry.Language); err != nil {
		return err
	}
	if !utf8.ValidString(entry.Text) || entry.Text == "" || len(entry.Text) > 65535 {
		return fmt.Errorf("%w: text must contain 1–65535 UTF-8 bytes", ErrInvalidEntry)
	}
	return nil
}

func dictionaryError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrConflict
	}
	return err
}

func (s *Store) Create(ctx context.Context, entry *Entry) error {
	if err := validateEntry(entry); err != nil {
		return err
	}
	entry.CreatedAt = time.Time{}
	entry.UpdatedAt = time.Time{}
	return dictionaryError(s.query(ctx).Clauses(clause.Insert{Table: clause.Table{Name: s.quotedTable(), Raw: true}}).Create(entry).Error)
}

func (s *Store) Get(ctx context.Context, messageID, language string) (*Entry, error) {
	language = normalizeLanguage(language)
	if err := validateKey(messageID, language); err != nil {
		return nil, err
	}
	var entry Entry
	err := s.query(ctx).Where("message_id = ? AND language = ?", messageID, language).Take(&entry).Error
	if err != nil {
		return nil, dictionaryError(err)
	}
	return &entry, nil
}

// Update changes only the translation text; the key is immutable.
func (s *Store) Update(ctx context.Context, entry *Entry) error {
	if err := validateEntry(entry); err != nil {
		return err
	}
	result := s.query(ctx).Where("message_id = ? AND language = ?", entry.MessageID, entry.Language).
		Updates(map[string]any{"text": entry.Text, "updated_at": time.Now()})
	if result.Error != nil {
		return dictionaryError(result.Error)
	}
	stored, err := s.Get(ctx, entry.MessageID, entry.Language)
	if err != nil {
		return err
	}
	*entry = *stored
	return nil
}

func (s *Store) Delete(ctx context.Context, messageID, language string) error {
	language = normalizeLanguage(language)
	if err := validateKey(messageID, language); err != nil {
		return err
	}
	result := s.query(ctx).Where("message_id = ? AND language = ?", messageID, language).Delete(&Entry{})
	if result.Error != nil {
		return dictionaryError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

type ListOptions struct {
	MessageID string
	Language  string
	Page      int
	PageSize  int
}

func (s *Store) List(ctx context.Context, options ListOptions) ([]Entry, int64, error) {
	entries := []Entry{}
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PageSize <= 0 {
		options.PageSize = 20
	}
	if options.Page > 1000000 || options.PageSize > 100 {
		return nil, 0, ErrInvalidEntry
	}
	q := s.query(ctx).Model(&Entry{})
	if options.MessageID != "" {
		q = q.Where("message_id = ?", options.MessageID)
	}
	if options.Language != "" {
		language := normalizeLanguage(options.Language)
		if len(language) > 35 || !languagePattern.MatchString(language) {
			return nil, 0, ErrInvalidEntry
		}
		q = q.Where("language = ?", language)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("message_id ASC, language ASC").Offset((options.Page - 1) * options.PageSize).
		Limit(options.PageSize).Find(&entries).Error
	return entries, total, err
}

// Translate falls back to the configured default language. A missing key returns ErrNotFound.
func (s *Store) Translate(ctx context.Context, messageID, language string) (string, error) {
	language = normalizeLanguage(language)
	if language == "" {
		language = s.defaultLanguage
	}
	entry, err := s.Get(ctx, messageID, language)
	if errors.Is(err, ErrNotFound) && language != s.defaultLanguage {
		entry, err = s.Get(ctx, messageID, s.defaultLanguage)
	}
	if err != nil {
		return "", err
	}
	return entry.Text, nil
}

func configuredStore() (*Store, error) {
	if !tablePattern.MatchString(config.Config.App.Name) {
		return nil, fmt.Errorf("i18n: invalid application name for dictionary table")
	}

	used := map[string]bool{}
	for _, name := range strings.Split(config.Config.Config.Used, ",") {
		used[strings.ToLower(strings.TrimSpace(name))] = true
	}
	var conn *gorm.DB
	var err error
	owned := false
	switch {
	case used["mysql"]:
		name := config.Config.GetConfigString("go.i18n.dbName")
		if db.Mysql.IsMultiDB() && name == "" {
			names := db.Mysql.ListConnNames()
			if len(names) == 0 {
				return nil, errors.New("i18n: MySQL has no available connections")
			}
			name = names[0]
		}
		if name != "" {
			conn, err = db.Mysql.GetConnection(name)
		} else {
			conn, err = db.Mysql.GetConnection()
		}
	case used["sqlite"]:
		conn, err = db.Sqlite.GetConnection()
	default:
		filename := config.Config.GetConfigString("go.i18n.sqlite")
		if filename == "" {
			filename = config.Config.App.Name + "_i18n.db"
		}
		if !filepath.IsAbs(filename) {
			filename = filepath.Join(config.Config.WorkDir, filename)
		}
		conn, err = gorm.Open(sqlite.Open(filename), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		owned = true
		if err == nil {
			pool, poolErr := conn.DB()
			if poolErr != nil {
				return nil, poolErr
			}
			pool.SetMaxOpenConns(1)
			pool.SetMaxIdleConns(1)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("i18n: opening dictionary database: %w", err)
	}
	s, err := NewStore(conn, config.Config.App.Name)
	if err != nil {
		if owned && conn != nil {
			if pool, e := conn.DB(); e == nil {
				_ = pool.Close()
			}
		}
		return nil, err
	}
	s.owned = owned
	language := normalizeLanguage(config.Config.GetConfigString("go.i18n.default"))
	if language != "" {
		if len(language) > 35 || !languagePattern.MatchString(language) {
			s.close()
			return nil, fmt.Errorf("%w: invalid default language", ErrInvalidEntry)
		}
		s.defaultLanguage = language
	}
	return s, nil
}

// Init initializes the built-in database dictionary; no external service is used.
func Init() error {
	initMu.Lock()
	defer initMu.Unlock()
	if currentStore.Load() != nil {
		return nil
	}
	s, err := configuredStore()
	if err != nil {
		return err
	}
	currentStore.Store(s)
	return nil
}

func GetStore() (*Store, error) {
	s := currentStore.Load()
	if s == nil {
		return nil, ErrNotInitialized
	}
	return s, nil
}

// Close releases only the SQLite connection owned by i18n.
func Close() {
	initMu.Lock()
	defer initMu.Unlock()
	if s := currentStore.Swap(nil); s != nil {
		s.close()
	}
}
func (s *Store) close() {
	if s.owned {
		if pool, err := s.conn.DB(); err == nil {
			_ = pool.Close()
		}
	}
}
