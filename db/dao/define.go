package dao

type Dao[E any] interface {
	Insert(entity *E) error
}
