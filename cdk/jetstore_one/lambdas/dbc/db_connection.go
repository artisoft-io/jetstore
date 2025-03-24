package dbc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Small component to hold a db connection that is refreshed when credential changes

// ENV VARIABLES:
// JETS_DSN_SECRET

type DbConnection struct {
	dbpool     *pgxpool.Pool
	dbPoolSize int
	lastCheck  time.Time
}

func NewDbConnection(poolSize int) (*DbConnection, error) {
	dbc := &DbConnection{
		dbPoolSize: poolSize,
		lastCheck:  time.Now(),
	}
	err := dbc.openDbConnection()
	return dbc, err
}

func (dbc *DbConnection) GetConnection() (*pgxpool.Pool, error) {
	if dbc.dbpool == nil {
		return nil, fmt.Errorf("error: database connection is already released")
	}
	if time.Since(dbc.lastCheck) > time.Minute*time.Duration(5) {
		// Check if dsn secret was rotated
		var sqltm sql.NullTime
		secretHasRotated := false
		err := dbc.dbpool.QueryRow(context.Background(), "SELECT MAX(last_update) FROM jetsapi.secret_rotation").Scan(&sqltm)
		if err != nil {
			switch {
			case strings.Contains(err.Error(), "password authentication failed"):
				secretHasRotated = true
			case !errors.Is(err, pgx.ErrNoRows):
				return nil, fmt.Errorf("while querying last_update from secret_rotation table: %v", err)
			}
		}
		if sqltm.Valid && dbc.lastCheck.Before(sqltm.Time) {
			secretHasRotated = true
		}
		dbc.lastCheck = time.Now()
		if secretHasRotated {
			// re-open db connection
			err = dbc.openDbConnection()
			if err != nil {
				return nil, fmt.Errorf("while re-opening db connection after secret rotation: %v", err)
			}
		}
	}
	return dbc.dbpool, nil
}

func (dbc *DbConnection) openDbConnection() error {
	// Get the dsn from the aws secret
	dsn, err := awsi.GetDsnFromSecret(os.Getenv("JETS_DSN_SECRET"), false, dbc.dbPoolSize)
	if err != nil {
		return fmt.Errorf("while getting dsn from aws secret: %v", err)
	}
	if dbc.dbpool != nil {
		dbc.dbpool.Close()
	}
	dbc.dbpool, err = pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	return nil
}

func (dbc *DbConnection) ReleaseConnection() {
	if dbc.dbpool != nil {
		dbc.dbpool.Close()
		dbc.dbpool = nil
	}
}
