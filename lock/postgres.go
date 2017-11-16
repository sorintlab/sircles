package lock

import (
	"context"
	"database/sql"
	"hash/fnv"

	"github.com/sorintlab/sircles/db"
)

type PGLockFactory struct {
	lockspace string
	db        *db.DB
}

func NewPGLockFactory(lockspace string, db *db.DB) *PGLockFactory {
	return &PGLockFactory{lockspace: lockspace, db: db}
}

func (l *PGLockFactory) NewLock(key string) Lock {
	return NewPGLock(l.db, l.lockspace, key)
}

type PGLock struct {
	db        *db.DB
	lockspace int32
	key       int32
	c         *sql.Conn
}

func NewPGLock(db *db.DB, lockspace, key string) *PGLock {
	return &PGLock{db: db, lockspace: hash(lockspace), key: hash(key)}
}

func (l *PGLock) Lock() error {
	if l.c != nil {
		panic("db connection isn't nil")
	}
	c, err := l.db.Conn()
	if err != nil {
		return err
	}
	_, err = c.ExecContext(context.TODO(), "select pg_advisory_lock($1, $2)", l.lockspace, l.key)
	if err != nil {
		c.Close()
		return err
	}
	l.c = c
	return nil

}

func (l *PGLock) Unlock() error {
	if l.c == nil {
		panic("db connection is nil")
	}
	_, _ = l.c.ExecContext(context.TODO(), "select pg_advisory_unlock($1, $2)", l.lockspace, l.key)
	_ = l.c.Close()
	l.c = nil
	return nil
}

func hash(s string) int32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int32(h.Sum32())
}
