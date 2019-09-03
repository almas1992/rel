// Package mysql wraps mysql driver as an adapter for grimoire.
//
// Usage:
//	// open mysql connection.
//	adapter, err := mysql.Open("root@(127.0.0.1:3306)/grimoire_test?charset=utf8&parseTime=True&loc=Local")
//	if err != nil {
//		panic(err)
//	}
//	defer adapter.Close()
//
//	// initialize grimoire's repo.
//	repo := grimoire.New(adapter)
package mysql

import (
	db "database/sql"
	"strings"

	"github.com/Fs02/go-paranoid"
	"github.com/Fs02/grimoire"
	"github.com/Fs02/grimoire/adapter/sql"
)

// Adapter definition for mysql database.
type Adapter struct {
	*sql.Adapter
}

var _ grimoire.Adapter = (*Adapter)(nil)

// Open mysql connection using dsn.
func Open(dsn string) (*Adapter, error) {
	var err error

	adapter := &Adapter{
		Adapter: &sql.Adapter{
			Config: &sql.Config{
				Placeholder:   "?",
				EscapeChar:    "`",
				IncrementFunc: incrementFunc,
				ErrorFunc:     errorFunc,
			},
		},
	}
	adapter.DB, err = db.Open("mysql", dsn)

	return adapter, err
}

func incrementFunc(adapter sql.Adapter) int {
	var variable string
	var increment int
	var err error
	if adapter.Tx != nil {
		err = adapter.Tx.QueryRow("SHOW VARIABLES LIKE 'auto_increment_increment';").Scan(&variable, &increment)
	} else {
		err = adapter.DB.QueryRow("SHOW VARIABLES LIKE 'auto_increment_increment';").Scan(&variable, &increment)
	}
	paranoid.Panic(err, "grimoire: MySQL failed to get auto_increment_increment variable")

	return increment
}

func errorFunc(err error) error {
	if err == nil {
		return nil
	}

	var (
		msg          = err.Error()
		errCodeSep   = ':'
		errCodeIndex = strings.IndexRune(msg, errCodeSep)
	)

	if errCodeIndex < 0 {
		errCodeIndex = 0
	}

	switch msg[:errCodeIndex] {
	case "Error 1062":
		return grimoire.ConstraintError{
			Key:  sql.ExtractString(msg, "key '", "'"),
			Type: grimoire.UniqueConstraint,
			Err:  err,
		}
	case "Error 1452":
		return grimoire.ConstraintError{
			Key:  sql.ExtractString(msg, "CONSTRAINT `", "`"),
			Type: grimoire.ForeignKeyConstraint,
			Err:  err,
		}
	default:
		return err
	}
}
