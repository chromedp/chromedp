package chromedp

import (
	"sync"

	"encoding/json/jsontext"
)

// subscribers holds the event subscriptions of a session, indexed by the
// method of the event.
type subscribers struct {
	mu     sync.Mutex
	subs   map[string]map[*subscription]struct{}
	closed bool
}

// subscribe adds a subscription for the events with the method. It buffers
// every event from the moment that it returns. The returned func stops the
// subscription and closes the channel.
func (s *subscribers) subscribe(method string) (<-chan jsontext.Value, func()) {
	sub := &subscription{
		out:  make(chan jsontext.Value),
		wake: make(chan struct{}, 1),
		done: make(chan struct{}),
	}
	s.mu.Lock()
	if s.closed {
		sub.stop()
	} else {
		if s.subs == nil {
			s.subs = make(map[string]map[*subscription]struct{})
		}
		if s.subs[method] == nil {
			s.subs[method] = make(map[*subscription]struct{})
		}
		s.subs[method][sub] = struct{}{}
	}
	s.mu.Unlock()
	go sub.pump()

	return sub.out, func() {
		s.mu.Lock()
		delete(s.subs[method], sub)
		s.mu.Unlock()
		sub.stop()
	}
}

// publish hands the raw parameters of an event to the subscriptions for the
// method. It never blocks.
func (s *subscribers) publish(method string, params jsontext.Value) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sub := range s.subs[method] {
		sub.push(params)
	}
}

// close stops all subscriptions, and makes later subscriptions end at once.
func (s *subscribers) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for _, m := range s.subs {
		for sub := range m {
			sub.stop()
		}
	}
	s.subs = nil
}

// subscription is one subscriber. It keeps the events in a queue without a
// limit, so that publishing never blocks the handling of the events of the
// browser, and a goroutine moves them to the channel of the subscriber.
type subscription struct {
	out  chan jsontext.Value
	wake chan struct{}
	done chan struct{}

	mu       sync.Mutex
	queue    []jsontext.Value
	stopOnce sync.Once
}

func (s *subscription) push(v jsontext.Value) {
	s.mu.Lock()
	s.queue = append(s.queue, v)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// stop ends the subscription. It is safe to call more than once.
func (s *subscription) stop() {
	s.stopOnce.Do(func() { close(s.done) })
}

// pump sends the queued events to the channel, until the subscription ends.
func (s *subscription) pump() {
	defer close(s.out)
	for {
		s.mu.Lock()
		queue := s.queue
		s.queue = nil
		s.mu.Unlock()

		for _, v := range queue {
			select {
			case s.out <- v:
			case <-s.done:
				return
			}
		}
		if len(queue) > 0 {
			continue
		}
		select {
		case <-s.wake:
		case <-s.done:
			return
		}
	}
}
