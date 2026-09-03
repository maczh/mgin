package model

import "time"

// Product 商品示例模型 (GORM)
type Product struct {
	Id        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"column:name" json:"name"`
	Price     float64   `gorm:"column:price" json:"price"`
	Stock     int       `gorm:"column:stock" json:"stock"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名
func (Product) TableName() string {
	return "product"
}
