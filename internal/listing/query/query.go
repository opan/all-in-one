package query

type QueryOptions interface {
	Commit() error
	Rollback() error
}

type Transaction interface{}
