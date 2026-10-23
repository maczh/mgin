package dao

import (
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type transactionUser struct {
	ID   int `gorm:"primaryKey"`
	Name string
}

func (transactionUser) TableName() string { return "transaction_users" }

func TestMySQLDaoWithTxUsesTransactionConnection(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.AutoMigrate(&transactionUser{}); err != nil {
		t.Fatal(err)
	}

	err = conn.Transaction(func(tx *gorm.DB) error {
		dao := (&MySQLDao[transactionUser]{}).WithTx(tx)
		if err := dao.Create(&transactionUser{Name: "rolled back"}); err != nil {
			return err
		}
		return errors.New("business failure")
	})
	if err == nil {
		t.Fatal("expected transaction failure")
	}

	var count int64
	if err = conn.Model(&transactionUser{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 after rollback", count)
	}
}
