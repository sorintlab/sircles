package lock

import (
	"fmt"
	"sync"
)

type LocalLocks struct {
	locks map[string]*sync.Mutex
	m     sync.Mutex
}

func NewLocalLocks() *LocalLocks {
	return &LocalLocks{locks: make(map[string]*sync.Mutex)}
}

func (ll *LocalLocks) lock(key string) {
	ll.m.Lock()
	l, ok := ll.locks[key]
	if !ok {
		l = &sync.Mutex{}
		ll.locks[key] = l
	}
	ll.m.Unlock()
	l.Lock()
}

func (ll *LocalLocks) unlock(key string) {
	ll.m.Lock()
	l, ok := ll.locks[key]
	if !ok {
		panic(fmt.Errorf("no mutex for key: %s", key))
	}
	ll.m.Unlock()
	l.Unlock()
}

type LocalLockFactory struct {
	ll *LocalLocks
}

func NewLocalLockFactory(ll *LocalLocks) *LocalLockFactory {
	return &LocalLockFactory{ll: ll}
}

func (l *LocalLockFactory) NewLock(key string) Lock {
	return &LocalLock{ll: l.ll, key: key}
}

type LocalLock struct {
	ll  *LocalLocks
	key string
}

func (l *LocalLock) Lock() error {
	l.ll.lock(l.key)
	return nil

}

func (l *LocalLock) Unlock() error {
	l.ll.unlock(l.key)
	return nil
}
