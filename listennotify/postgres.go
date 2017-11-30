package listennotify

import (
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/sorintlab/sircles/db"
)

type PGNotifier struct {
	tx *db.Tx
}

func NewPGNotifier() *PGNotifier {
	return &PGNotifier{}
}

func (l *PGNotifier) BindTx(tx *db.Tx) {
	l.tx = tx
}

func (l *PGNotifier) Notify(channel string, payload string) error {
	if l.tx == nil {
		return errors.New("nil tx")
	}
	err := l.tx.Do(func(tx *db.WrappedTx) error {
		_, err := tx.Exec(fmt.Sprintf("notify %s %s", channel, payload))
		return err
	})
	return err
}

type PGNotifierFactory struct{}

func NewPGNotifierFactory() *PGNotifierFactory {
	return &PGNotifierFactory{}
}

func (lnf *PGNotifierFactory) NewNotifier() Notifier {
	return NewPGNotifier()
}

type PGListener struct {
	listener *pq.Listener
	notify   chan *Notification
	stop     chan struct{}
}

func NewPGListener(connString string) *PGListener {
	minReconn := 10 * time.Second
	maxReconn := time.Minute
	listener := pq.NewListener(connString, minReconn, maxReconn, nil)
	stop := make(chan struct{})
	notify := make(chan *Notification)
	l := &PGListener{listener: listener, notify: notify, stop: stop}

	go func() {
		for {
			select {
			case pn := <-l.listener.Notify:
				if pn == nil {
					continue
				}
				n := &Notification{
					Channel: pn.Channel,
					Payload: pn.Extra,
				}
				notify <- n

			case <-stop:
				return
			}
		}
	}()

	return l
}

func (l *PGListener) NotificationChannel() chan *Notification {
	return l.notify
}

func (l *PGListener) Listen(channel string) error {
	return l.listener.Listen(channel)
}

func (l *PGListener) Ping() error {
	return l.listener.Ping()
}

func (l *PGListener) Close() error {
	close(l.stop)
	return l.listener.Close()
}

type PGListenerFactory struct {
	connString string
}

func NewPGListenerFactory(connString string) *PGListenerFactory {
	return &PGListenerFactory{connString: connString}
}

func (lnf *PGListenerFactory) NewListener() Listener {
	return NewPGListener(lnf.connString)
}
