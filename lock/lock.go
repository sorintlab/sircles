package lock

type LockFactory interface {
	NewLock(key string) Lock
}

type Lock interface {
	Lock() error
	Unlock() error
}
