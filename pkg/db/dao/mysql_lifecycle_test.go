package dao

import (
	"testing"

	"gorm.io/gorm"
)

type invalidConnectionEntity struct {
	ID int
}

func (invalidConnectionEntity) TableName() string { return "invalid_connection_entities" }

func TestAllRejectsUninitializedConnection(t *testing.T) {
	rowDao := &MySQLDao[invalidConnectionEntity]{tx: &gorm.DB{}}
	result, err := rowDao.All(invalidConnectionEntity{})
	if err == nil {
		t.Fatal("expected an error for an uninitialized connection")
	}
	if result != nil {
		t.Fatal("expected no result for an uninitialized connection")
	}
}

func TestJoinRejectsUninitializedConnection(t *testing.T) {
	rowDao := &MySQLDao[invalidConnectionEntity]{tx: &gorm.DB{}}
	if conn := rowDao.JOIN("related", "JOIN related ON related.id = invalid_connection_entities.id"); conn != nil {
		t.Fatal("expected no connection for an uninitialized join")
	}
}
