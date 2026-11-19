package orm

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// IDB is what *bun.DB, bun.Tx and bun.Conn have in common. Repositories take
// an IDB, so the same code runs on the pool and inside a transaction.
type IDB = bun.IDB

// Options are the connection settings. They are plain values: the caller
// reads them from wherever it keeps its configuration.
type Options struct {
	Host     string
	Port     int // 5432 when zero
	Database string
	User     string
	Password string
	// Schema becomes the search_path when set.
	Schema string
	// SSLMode is disable, prefer (when empty), require, verify-ca or verify-full.
	SSLMode string
	// ApplicationName shows up in pg_stat_activity.
	ApplicationName string

	// ConnectTimeout bounds opening one connection; none when zero.
	ConnectTimeout time.Duration
	// MaxOpenConns limits open connections; unlimited when zero.
	MaxOpenConns int
	// MaxIdleConns is how many idle connections are kept; database/sql's
	// default (2) when zero.
	MaxIdleConns int
	// ConnMaxLifetime closes connections older than this; never when zero.
	ConnMaxLifetime time.Duration
	// ConnMaxIdleTime closes connections idle longer than this; never when zero.
	ConnMaxIdleTime time.Duration
}

// DSN is the connection URL of o, password included: never log it.
func (o Options) DSN() string {
	port := o.Port
	if port == 0 {
		port = 5432
	}
	sslMode := o.SSLMode
	if sslMode == "" {
		sslMode = "prefer"
	}
	query := url.Values{}
	query.Set("sslmode", sslMode)
	if o.Schema != "" {
		query.Set("search_path", o.Schema)
	}
	if o.ApplicationName != "" {
		query.Set("application_name", o.ApplicationName)
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(o.User, o.Password),
		Host:     net.JoinHostPort(o.Host, strconv.Itoa(port)),
		Path:     "/" + o.Database,
		RawQuery: query.Encode(),
	}
	return u.String()
}

// Open connects to PostgreSQL through pgx and checks the connection with a
// ping. Close the returned DB when done; its embedded *sql.DB is available
// as db.DB for code that wants database/sql.
func Open(ctx context.Context, o Options) (*bun.DB, error) {
	// Host, port and sslmode go through the URL: pgx turns sslmode=prefer into
	// a TLS attempt plus a plain-text fallback, and fields set on the parsed
	// config afterwards would not reach the fallback.
	config, err := pgx.ParseConfig(o.DSN())
	if err != nil {
		// The error would quote the DSN, password included.
		return nil, fmt.Errorf("orm: invalid settings for %s:%d", o.Host, o.Port)
	}
	if o.ConnectTimeout > 0 {
		config.ConnectTimeout = o.ConnectTimeout
	}
	// bun writes the arguments into the SQL text itself, so almost every
	// query is a new statement: the simple protocol sends it as is instead of
	// preparing and caching each one.
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	sqldb := stdlib.OpenDB(*config)
	if o.MaxOpenConns > 0 {
		sqldb.SetMaxOpenConns(o.MaxOpenConns)
	}
	if o.MaxIdleConns > 0 {
		sqldb.SetMaxIdleConns(o.MaxIdleConns)
	}
	sqldb.SetConnMaxLifetime(o.ConnMaxLifetime)
	sqldb.SetConnMaxIdleTime(o.ConnMaxIdleTime)

	db := NewDB(sqldb)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("orm: ping %s:%d: %w", o.Host, o.Port, err)
	}
	return db, nil
}

// NewDB wraps an already opened *sql.DB, e.g. one a test built, in bun with
// the PostgreSQL dialect.
func NewDB(sqldb *sql.DB) *bun.DB {
	return bun.NewDB(sqldb, pgdialect.New())
}
