package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
)

// InTx runs fn in a transaction on db. It commits when fn returns nil and
// rolls back when fn returns an error or panics. Build the repositories fn
// needs on tx:
//
//	err := orm.InTx(ctx, db, func(ctx context.Context, tx orm.IDB) error {
//		if err := invoices.WithTx(tx).Create(ctx, invoice); err != nil {
//			return err
//		}
//		return items.WithTx(tx).CreateMany(ctx, lines)
//	})
//
// When db is already a transaction, fn runs in a savepoint of it.
func InTx(ctx context.Context, db IDB, fn func(ctx context.Context, tx IDB) error) error {
	return InTxOptions(ctx, db, nil, fn)
}

// InTxOptions is InTx with an isolation level or a read-only transaction.
func InTxOptions(ctx context.Context, db IDB, opts *sql.TxOptions, fn func(ctx context.Context, tx IDB) error) error {
	return db.RunInTx(ctx, opts, func(ctx context.Context, tx bun.Tx) error {
		return fn(ctx, tx)
	})
}
