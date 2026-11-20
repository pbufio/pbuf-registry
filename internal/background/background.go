package background

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	CompactionLockID   int64 = 0x70627566636f6d70 // "pbufcomp"
	ProtoParsingLockID int64 = 0x7062756670617273 // "pbufpars"
)

type Daemon interface {
	Name() string
	Run(ctx context.Context) error
}

type advisoryLockedDaemon struct {
	pool   *pgxpool.Pool
	lockID int64
	inner  Daemon
	log    *log.Helper
}

// WithAdvisoryLock ensures at most one replica or --once invocation runs the daemon against PostgreSQL at a time.
func WithAdvisoryLock(pool *pgxpool.Pool, lockID int64, daemon Daemon, logger log.Logger) Daemon {
	if pool == nil {
		return daemon
	}
	return &advisoryLockedDaemon{
		pool:   pool,
		lockID: lockID,
		inner:  daemon,
		log:    log.NewHelper(log.With(logger, "module", "background/AdvisoryLock")),
	}
}

func (d *advisoryLockedDaemon) Name() string {
	return d.inner.Name()
}

func (d *advisoryLockedDaemon) Run(ctx context.Context) error {
	if maxConns := d.pool.Config().MaxConns; maxConns < 2 {
		return fmt.Errorf("database pool max_conns (%d) must be at least 2 for %s advisory lock", maxConns, d.inner.Name())
	}
	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire database connection for %s lock: %w", d.inner.Name(), err)
	}

	var acquired bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", d.lockID).Scan(&acquired); err != nil {
		conn.Release()
		return fmt.Errorf("failed to acquire advisory lock for %s: %w", d.inner.Name(), err)
	}
	if !acquired {
		conn.Release()
		d.log.Infof("Skipping %s run: another instance holds advisory lock", d.inner.Name())
		return nil
	}

	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		if err := conn.QueryRow(unlockCtx, "SELECT pg_advisory_unlock($1)", d.lockID).Scan(&unlocked); err != nil || !unlocked {
			d.log.Warnf("Failed to release advisory lock for %s cleanly (err=%v, unlocked=%v), closing session", d.inner.Name(), err, unlocked)
			_ = conn.Conn().Close(unlockCtx)
		}
		conn.Release()
	}()

	return d.inner.Run(ctx)
}
