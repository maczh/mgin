package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/examples/quickstart/service"
)

var productService = &service.ProductService{}

// ListProducts GET /api/v1/products
func ListProducts(c *gin.Context) {
	list, err := productService.List()
	if err != nil {
		c.JSON(200, models.Error(500, err.Error()))
		return
	}
	c.JSON(200, models.Success(list))
}

// GetProduct GET /api/v1/products/:id
func GetProduct(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(200, models.Error(400, "参数错误"))
		return
	}
	product, err := productService.Get(id)
	if err != nil {
		c.JSON(200, models.Error(500, err.Error()))
		return
	}
	if product == nil {
		c.JSON(200, models.Error(-1, "商品不存在"))
		return
	}
	c.JSON(200, models.Success(product))
}
