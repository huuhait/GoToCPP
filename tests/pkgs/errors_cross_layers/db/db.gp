package db

import "errors"

var ErrNoRows = errors.New("db: no rows")

type QueryError struct {
	Query string
	Err   error
}

func (q *QueryError) Error() string { return "query " + q.Query + ": " + q.Err.Error() }
func (q *QueryError) Unwrap() error { return q.Err }

func Find(id int) error {
	if id < 0 {
		return &QueryError{"find", ErrNoRows}
	}
	return nil
}
