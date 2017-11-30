package listennotify

import "github.com/sorintlab/sircles/db"

type Type int

const (
	Local Type = iota
	Postgres
)

type Notification struct {
	Channel string
	Payload string
}

type Notifier interface {
	Notify(channel string, payload string) error
}

type TxNotifier interface {
	Notifier
	BindTx(tx *db.Tx)
}

type NotifierFactory interface {
	NewNotifier() Notifier
}

type Listener interface {
	NotificationChannel() chan *Notification
	Close() error
	Listen(channel string) error
	Ping() error
}

type ListenerFactory interface {
	NewListener() Listener
}
