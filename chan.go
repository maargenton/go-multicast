package multicast

import (
	"sync"
)

// Chan sends every incoming message to each subscription channel
type Chan struct {
	mu            sync.Mutex       // protects close(in) and closed flag
	in            chan interface{} // closed when client is donr writing
	closed        bool             // indicate that the in channel is already closed
	closeCh       chan struct{}    // closed when runloop is exiting
	subscriptions map[*Subscription]struct{}
	subscribeCh   chan *Subscription
	unsubscribeCh chan *Subscription
}

// NewChan returns a new muticast channel with an unbuffered input channel
func NewChan() *Chan {
	return newChan(0)
}

// NewBufferedChan returns a new muticast channel with a buffered input channel
// of the specified size
func NewBufferedChan(bufferSize int) *Chan {
	return newChan(bufferSize)
}

// In return the input channel to which items are sent to
func (ch *Chan) In() chan<- interface{} {
	return ch.in
}

// Subscribe returns a new unbuferred sbscription channel that will receive
// every input item sent to the multicast channel
func (ch *Chan) Subscribe() *Subscription {
	return newSunscription(ch, 0)
}

// SubscribeBuffered returns a new buferred sbscription channel that will
// receive every input item sent to the multicast channel
func (ch *Chan) SubscribeBuffered(bufferSize int) *Subscription {
	return newSunscription(ch, bufferSize)
}

// Close closes the multicast channel
func (ch *Chan) Close() error {
	ch.close()
	return nil
}

// ---------------------------------------------------------------------------
// Chan internals
// ---------------------------------------------------------------------------

func newChan(bufferSize int) *Chan {
	ch := &Chan{
		in:            make(chan interface{}, bufferSize),
		closeCh:       make(chan struct{}),
		subscriptions: make(map[*Subscription]struct{}),
		subscribeCh:   make(chan *Subscription),
		unsubscribeCh: make(chan *Subscription),
	}
	go ch.runloop()
	return ch
}

func (ch *Chan) close() {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if !ch.closed {
		ch.closed = true
		close(ch.in)
	}
}

func (ch *Chan) runloop() {
	pendingClose := make([]*Subscription, 0)
loop:
	for {
		select {
		case v, ok := <-ch.in:
			if ok {
				for s := range ch.subscriptions {
					if !s.write(v) {
						pendingClose = append(pendingClose, s)
					}
				}
				for _, s := range pendingClose {
					delete(ch.subscriptions, s)
					s.closeReadChannel()
				}
				pendingClose = pendingClose[:0]

			} else {
				break loop
			}
		case s := <-ch.subscribeCh:
			ch.subscriptions[s] = struct{}{}

		case s := <-ch.unsubscribeCh:
			delete(ch.subscriptions, s)
			s.closeReadChannel()
		}
	}
	close(ch.closeCh)
	for s := range ch.subscriptions {
		s.closeReadChannel()
	}
}

// ---------------------------------------------------------------------------
// Subscription
// ---------------------------------------------------------------------------

// Subscription is the receiver channel of a Subscription
type Subscription struct {
	mu      sync.Mutex // Protect the closing of the channels
	ch      *Chan      // Set ot nil when readCh is closed
	closeCh chan struct{}
	readCh  chan interface{}
}

// Close closes the subscription
func (s *Subscription) Close() error {
	s.unsubscribe()
	return nil
}

// Out returns the read channel for the subscription
func (s *Subscription) Out() <-chan interface{} {
	return s.readCh
}

// ---------------------------------------------------------------------------
// Subscription internals
// ---------------------------------------------------------------------------

func newSunscription(ch *Chan, bufferSize int) *Subscription {
	s := &Subscription{
		ch:      ch,
		closeCh: make(chan struct{}),
		readCh:  make(chan interface{}, bufferSize),
	}

	if s.ch == nil {
		close(s.closeCh)
		close(s.readCh)
	} else {
		select {
		case ch.subscribeCh <- s:
		case <-ch.closeCh:
			s.ch = nil
			close(s.closeCh)
			close(s.readCh)
		}
	}
	return s
}

func (s *Subscription) write(v interface{}) bool {
	select {
	case <-s.closeCh:
		return false
	case s.readCh <- v:
		return true
	}
}

func (s *Subscription) unsubscribe() {

	s.mu.Lock()
	select {
	case <-s.closeCh:
	default:
		close(s.closeCh) // handles concurrent write / close
	}
	ch := s.ch
	s.mu.Unlock()

	// schedule close readCh on writer side
	if ch != nil {
		select {
		case ch.unsubscribeCh <- s:
		case <-ch.closeCh:
			s.mu.Lock()
			if s.ch != nil {
				s.ch = nil
				close(s.readCh)
			}
			s.mu.Unlock()
		}
	}
}

func (s *Subscription) closeReadChannel() {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.closeCh:
	default:
		close(s.closeCh)
	}
	if s.ch != nil {
		s.ch = nil
		close(s.readCh)
	}
}
