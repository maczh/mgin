package db

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type transactionRecord struct {
	ID    int `gorm:"primaryKey"`
	Value string
}

func TestTransactionOnCommitAndRollback(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.AutoMigrate(&transactionRecord{}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		callback   func(*gorm.DB) (int, error)
		wantCount  int64
		wantResult int
		wantError  bool
	}{
		{
			name: "commit",
			callback: func(tx *gorm.DB) (int, error) {
				if err := tx.Create(&transactionRecord{Value: "committed"}).Error; err != nil {
					return 0, err
				}
				return 42, nil
			},
			wantCount: 1, wantResult: 42,
		},
		{
			name: "rollback",
			callback: func(tx *gorm.DB) (int, error) {
				if err := tx.Create(&transactionRecord{Value: "rolled back"}).Error; err != nil {
					return 0, err
				}
				return 0, errors.New("business failure")
			},
			wantCount: 1, wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TransactionOn(context.Background(), conn, tt.callback)
			if (err != nil) != tt.wantError {
				t.Fatalf("error = %v, wantError %v", err, tt.wantError)
			}
			if result != tt.wantResult {
				t.Fatalf("result = %d, want %d", result, tt.wantResult)
			}
			var count int64
			if err = conn.Model(&transactionRecord{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != tt.wantCount {
				t.Fatalf("count = %d, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestTransactionOnRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct {
		name string
		conn *gorm.DB
		fn   func(*gorm.DB) (string, error)
	}{
		{name: "nil connection", fn: func(*gorm.DB) (string, error) { return "", nil }},
		{name: "nil callback", conn: &gorm.DB{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := TransactionOn(context.Background(), test.conn, test.fn)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
