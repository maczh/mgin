package dao

import "gorm.io/gorm"

// Dao[E] 是 mgin v2 扩展后的泛型 DAO 接口。
//
// v1 时代仅包含 Insert(*E) error 一个方法，v2 扩展为与 db/dao/mysql.go、postgres.go、
// clickhouse.go、mgo.go 等具体实现对齐的最小方法集。已有具体实现（MySQLDao 等）的方法
// 集合已经超过这个接口，因此把它们"补齐签名"以满足本接口即可。
//
// 注意：v2 Dao 接口仍然保持小而精，避免在接口里强加大量可选方法。具体实现可以
// 拥有接口之外的扩展方法（如 MySQLDao.Debug()、MySQLDao.WithContext()），
// 业务需要时直接断言到具体类型调用。
type Dao[E any] interface {
	Insert(entity *E) error
	Where(query interface{}, args ...interface{}) *gorm.DB
	Find(query interface{}, args ...interface{}) ([]E, error)
	FindById(id interface{}) (E, error)
	Update(entity *E) error
	Delete(entity *E) error
	Count() (int64, error)
}
