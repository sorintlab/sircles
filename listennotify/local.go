package listennotify

import (
	"sync"
)

type LocalListenNotify struct {
	channels map[string][]chan *Notification

	rwMutex sync.RWMutex
}

func (ln *LocalListenNotify) start(channel string, c chan *Notification) {
	ln.rwMutex.Lock()
	defer ln.rwMutex.Unlock()

	ln.channels[channel] = append(ln.channels[channel], c)
}

func (ln *LocalListenNotify) stop(channel string, c chan *Notification) {
	ln.rwMutex.Lock()
	defer ln.rwMutex.Unlock()

	newArray := make([]chan *Notification, 0)
	outChans, ok := ln.channels[channel]
	if !ok {
		return
	}
	for _, ch := range outChans {
		if ch != c {
			newArray = append(newArray, ch)
		}
	}
	ln.channels[channel] = newArray

	return
}

func (ln *LocalListenNotify) stopAll(c chan *Notification) {
	ln.rwMutex.Lock()
	defer ln.rwMutex.Unlock()

	for channel := range ln.channels {
		newArray := make([]chan *Notification, 0)
		outChans, ok := ln.channels[channel]
		if !ok {
			return
		}
		for _, ch := range outChans {
			if ch != c {
				newArray = append(newArray, ch)
			}
		}
		ln.channels[channel] = newArray
	}

	return
}

func (ln *LocalListenNotify) notify(channel string, payload string) {
	ln.rwMutex.RLock()
	defer ln.rwMutex.RUnlock()

	outChans, ok := ln.channels[channel]
	if !ok {
		return
	}
	for _, outputChan := range outChans {
		outputChan <- &Notification{
			Channel: channel,
			Payload: payload,
		}
	}
}

func NewLocalListenNotify() *LocalListenNotify {
	return &LocalListenNotify{
		channels: make(map[string][]chan *Notification),
	}
}

type LocalNotifier struct {
	ln *LocalListenNotify
}

func (l *LocalNotifier) Notify(channel string, payload string) error {
	l.ln.notify(channel, payload)
	return nil
}

type LocalNotifierFactory struct {
	ln *LocalListenNotify
}

func NewLocalNotifierFactory(ln *LocalListenNotify) *LocalNotifierFactory {
	return &LocalNotifierFactory{ln: ln}
}

func (lnf *LocalNotifierFactory) NewNotifier() Notifier {
	return &LocalNotifier{
		ln: lnf.ln,
	}
}

type LocalListener struct {
	ln     *LocalListenNotify
	notify chan *Notification
	stop   chan struct{}
}

func (l *LocalListener) NotificationChannel() chan *Notification {
	return l.notify
}

func (l *LocalListener) Listen(channel string) error {
	l.ln.start(channel, l.notify)
	return nil
}

func (l *LocalListener) Ping() error {
	return nil
}

func (l *LocalListener) Close() error {
	l.ln.stopAll(l.notify)
	close(l.notify)
	return nil
}

type LocalListenerFactory struct {
	ln *LocalListenNotify
}

func NewLocalListenerFactory(ln *LocalListenNotify) *LocalListenerFactory {
	return &LocalListenerFactory{ln: ln}
}

func (lnf *LocalListenerFactory) NewListener() Listener {
	stop := make(chan struct{})
	notify := make(chan *Notification, 1000)
	return &LocalListener{
		ln:     lnf.ln,
		notify: notify,
		stop:   stop,
	}
}
