package chromedp

import (
	jsonv2 "github.com/chromedp/cdproto/cdp/jsonv2"
	"strconv"
	"sync"
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
func (s *subscribers) subscribe(method string) (<-chan jsonv2.Value, func()) {
	sub := &subscription{
		out:  make(chan jsonv2.Value),
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

// subscribeMany is like subscribe for several methods, with one queue. The
// channel gets the events of all the methods in the order that they arrived.
// Each value is a JSON object with the fields method and params, because a
// reader cannot tell the method from the parameters alone.
func (s *subscribers) subscribeMany(methods ...string) (<-chan jsonv2.Value, func()) {
	sub := &subscription{
		out:    make(chan jsonv2.Value),
		wake:   make(chan struct{}, 1),
		done:   make(chan struct{}),
		tagged: true,
	}
	s.mu.Lock()
	if s.closed {
		sub.stop()
	} else {
		if s.subs == nil {
			s.subs = make(map[string]map[*subscription]struct{})
		}
		for _, method := range methods {
			if s.subs[method] == nil {
				s.subs[method] = make(map[*subscription]struct{})
			}
			s.subs[method][sub] = struct{}{}
		}
	}
	s.mu.Unlock()
	go sub.pump()

	return sub.out, func() {
		s.mu.Lock()
		for _, method := range methods {
			delete(s.subs[method], sub)
		}
		s.mu.Unlock()
		sub.stop()
	}
}

// publish hands the raw parameters of an event to the subscriptions for the
// method. It never blocks.
func (s *subscribers) publish(method string, params jsonv2.Value) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sub := range s.subs[method] {
		if sub.tagged {
			sub.push(tagEvent(method, params))
			continue
		}
		sub.push(params)
	}
}

// tagEvent wraps the parameters of an event in a JSON object with the method.
func tagEvent(method string, params jsonv2.Value) jsonv2.Value {
	b := append([]byte(`{"method":`), strconv.Quote(method)...)
	b = append(b, `,"params":`...)
	if len(params) == 0 {
		params = jsonv2.Value("null")
	}
	b = append(b, params...)
	return append(b, '}')
}

// close ends all subscriptions, and makes later subscriptions end at once. A
// subscription first delivers the events that it already holds, so that a
// reader sees every event up to the end of the session.
func (s *subscribers) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for _, m := range s.subs {
		for sub := range m {
			sub.finish()
		}
	}
	s.subs = nil
}

// subscription is one subscriber. It keeps the events in a queue without a
// limit, so that publishing never blocks the handling of the browser events.
// A goroutine moves the events to the channel of the subscriber.
type subscription struct {
	out  chan jsonv2.Value
	wake chan struct{}
	done chan struct{}

	// tagged makes publish wrap each event with its method.
	tagged bool

	mu       sync.Mutex
	queue    []jsonv2.Value
	finished bool
	stopOnce sync.Once
}

func (s *subscription) push(v jsonv2.Value) {
	s.mu.Lock()
	s.queue = append(s.queue, v)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// finish ends the subscription after it delivered the queued events.
func (s *subscription) finish() {
	s.mu.Lock()
	s.finished = true
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// stop ends the subscription at once, and drops the queued events. It is safe
// to call more than once.
func (s *subscription) stop() {
	s.stopOnce.Do(func() { close(s.done) })
}

// pump sends the queued events to the channel, until the subscription ends.
func (s *subscription) pump() {
	defer close(s.out)
	for {
		s.mu.Lock()
		queue, finished := s.queue, s.finished
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
		if finished {
			return
		}
		select {
		case <-s.wake:
		case <-s.done:
			return
		}
	}
}
