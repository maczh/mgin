package service

import (
	"github.com/maczh/mgin/examples/quickstart/model"
)

// ProductService 商品业务层
// 示例采用内存数据；接入数据库时, 将本文件替换为基于 github.com/maczh/mgin/examples/quickstart/dao 的实现即可。
type ProductService struct{}

var mockProducts = []model.Product{
	{Id: 1, Name: "示例商品A", Price: 99.9, Stock: 100},
	{Id: 2, Name: "示例商品B", Price: 199.9, Stock: 50},
}

// List 查询商品列表
func (s *ProductService) List() ([]model.Product, error) {
	return mockProducts, nil
}

// Get 根据 ID 查询商品
func (s *ProductService) Get(id int64) (*model.Product, error) {
	for i := range mockProducts {
		if mockProducts[i].Id == id {
			return &mockProducts[i], nil
		}
	}
	return nil, nil
}
