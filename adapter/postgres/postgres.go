// Package postgres wraps postgres (pq) driver as an adapter for grimoire.
//
// Usage:
//	// open postgres connection.
//	adapter, err := postgres.Open("postgres://postgres@localhost/grimoire_test?sslmode=disable")
//	if err != nil {
//		panic(err)
//	}
//	defer adapter.Close()
//
//	// initialize grimoire's repo.
//	repo := grimoire.New(adapter)
package postgres

import (
	db "database/sql"
	"time"

	"github.com/Fs02/grimoire"
	"github.com/Fs02/grimoire/adapter/sql"
	"github.com/lib/pq"
)

// Adapter definition for postgrees database.
type Adapter struct {
	*sql.Adapter
}

var _ grimoire.Adapter = (*Adapter)(nil)

// Open postgrees connection using dsn.
func Open(dsn string) (*Adapter, error) {
	var err error

	adapter := &Adapter{
		Adapter: &sql.Adapter{
			Config: &sql.Config{
				Placeholder:         "$",
				EscapeChar:          "\"",
				Ordinal:             true,
				InsertDefaultValues: true,
				ErrorFunc:           errorFunc,
			},
		},
	}
	adapter.DB, err = db.Open("postgres", dsn)

	return adapter, err
}

// Insert inserts a record to database and returns its id.
func (adapter *Adapter) Insert(query grimoire.Query, changes grimoire.Changes, loggers ...grimoire.Logger) (interface{}, error) {
	var (
		id              int64
		statement, args = sql.NewBuilder(adapter.Config).Returning("id").Insert(query.Collection, changes)
		rows, err       = adapter.query(statement, args, loggers)
	)

	if err == nil && rows.Next() {
		rows.Scan(&id)
	}

	return id, err
}

// InsertAll inserts multiple records to database and returns its ids.
func (adapter *Adapter) InsertAll(query grimoire.Query, fields []string, allchanges []grimoire.Changes, loggers ...grimoire.Logger) ([]interface{}, error) {
	var (
		ids             []interface{}
		statement, args = sql.NewBuilder(adapter.Config).Returning("id").InsertAll(query.Collection, fields, allchanges)
		rows, err       = adapter.query(statement, args, loggers)
	)

	if err == nil {
		for rows.Next() {
			var id int64
			rows.Scan(&id)
			ids = append(ids, id)
		}
	}

	return ids, err
}

func (adapter *Adapter) query(statement string, args []interface{}, loggers []grimoire.Logger) (*db.Rows, error) {
	var (
		err   error
		rows  *db.Rows
		start = time.Now()
	)

	if adapter.Tx != nil {
		rows, err = adapter.Tx.Query(statement, args...)
	} else {
		rows, err = adapter.DB.Query(statement, args...)
	}

	go grimoire.Log(loggers, statement, time.Since(start), err)

	return rows, errorFunc(err)
}

// Begin begins a new transaction.
func (adapter *Adapter) Begin() (grimoire.Adapter, error) {
	newAdapter, err := adapter.Adapter.Begin()

	return &Adapter{
		Adapter: newAdapter.(*sql.Adapter),
	}, err
}

func errorFunc(err error) error {
	if err == nil {
		return nil
	}

	if e, ok := err.(*pq.Error); ok {
		switch e.Code {
		case "23505":
			return grimoire.ConstraintError{
				Key:  sql.ExtractString(err.Error(), "constraint \"", "\""),
				Type: grimoire.UniqueConstraint,
				Err:  err,
			}
		case "23503":
			return grimoire.ConstraintError{
				Key:  sql.ExtractString(err.Error(), "constraint \"", "\""),
				Type: grimoire.ForeignKeyConstraint,
				Err:  err,
			}
		case "23514":
			return grimoire.ConstraintError{
				Key:  sql.ExtractString(err.Error(), "constraint \"", "\""),
				Type: grimoire.CheckConstraint,
				Err:  err,
			}
		}
	}

	return err
}
