package eventhandler

import (
	"time"

	ln "github.com/sorintlab/sircles/listennotify"
	"github.com/sorintlab/sircles/lock"
)

type EventHandler interface {
	HandleEvents() error
	Name() string
}

func RunEventHandler(eh EventHandler, stop chan struct{}, lnf ln.ListenerFactory, lkf lock.LockFactory) (chan struct{}, error) {
	l := lnf.NewListener()

	if err := l.Listen("event"); err != nil {
		return nil, err
	}

	nCh := l.NotificationChannel()
	endCh := make(chan struct{})

	go func() {
		for {
			// Take a distributed lock to avoid multiple instances handling the
			// same events. Note that this won't create real issues but various
			// errors could be logged since one of the instances will fail to
			// handle events already handled by the other instance (i.e.
			// concurrent update errors on the event stream, unique violations
			// in the readdb etc...)
			lk := lkf.NewLock(eh.Name())
			if err := lk.Lock(); err != nil {
				log.Errorf("failed to acquire lock: %+v", err)
			} else {
				if err := eh.HandleEvents(); err != nil {
					log.Errorf("eventhandler HandleEvents error: %+v", err)
				}
				lk.Unlock()
			}
			select {
			case <-nCh:
				continue

			case <-time.After(10 * time.Second):
				go l.Ping()
				continue

			case <-stop:
				l.Close()
				close(endCh)
				return
			}
		}
	}()

	return endCh, nil
}
