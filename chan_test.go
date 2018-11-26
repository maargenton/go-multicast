package multicast_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marcus999/go-multicast"
	"github.com/marcus999/go-testpredicate"
	"github.com/marcus999/go-testpredicate/pred"
)

// ----------------------------------------------------------------------------
// Tests for constructors
// ----------------------------------------------------------------------------

func TestNewChan(t *testing.T) {
	assert := testpredicate.NewAsserter(t)

	ch := multicast.NewChan()
	assert.That(ch, pred.IsNotNil())
	assert.That(ch.In(), pred.IsNotNil(), "multicast.Chan must have a valid input channel")
}

func TestNewBufferredChan(t *testing.T) {
	assert := testpredicate.NewAsserter(t)

	ch := multicast.NewBufferedChan(10)
	assert.That(ch, pred.IsNotNil())
	assert.That(ch.In(), pred.IsNotNil(), "multicast.Chan must have a valid input channel")
}

// ----------------------------------------------------------------------------
// Helper functions
// ----------------------------------------------------------------------------

type AtomicBool struct {
	i int32
}

func (b *AtomicBool) Get() bool {

	return atomic.LoadInt32(&b.i) != 0
}

func (b *AtomicBool) Set(v bool) {
	var i int32
	if v {
		i = 1
	}
	atomic.StoreInt32(&b.i, i)
}

func makeData(start, count int) []interface{} {
	result := make([]interface{}, 0, count)
	for i := 0; i < count; i++ {
		result = append(result, start+i)
	}

	return result
}

func feedChan(values []interface{}, ch chan<- interface{}) {
	for _, e := range values {
		ch <- e
	}
}

func drainChan(ch <-chan interface{}) []interface{} {
	result := make([]interface{}, 0, 16)
	for e := range ch {
		result = append(result, e)
	}
	return result
}

func drainChanN(ch <-chan interface{}, count int) []interface{} {
	result := make([]interface{}, 0, 16)
	for e := range ch {
		result = append(result, e)
		if len(result) >= count {
			break
		}
	}
	return result
}

// ----------------------------------------------------------------------------
// Tests for channel and subscription operation
// ----------------------------------------------------------------------------

func TestElementsAreDispatchedToAllSubscribers(t *testing.T) {
	assert := testpredicate.NewAsserter(t)

	ch := multicast.NewChan()
	s1 := ch.SubscribeBuffered(10)
	s2 := ch.SubscribeBuffered(10)
	s3 := ch.SubscribeBuffered(10)

	data := makeData(0, 10)
	feedChan(data, ch.In())
	ch.Close()
	r1 := drainChan(s1.Out())
	r2 := drainChan(s2.Out())
	r3 := drainChan(s3.Out())

	assert.That(r1, pred.IsEqualTo(data))
	assert.That(r2, pred.IsEqualTo(data))
	assert.That(r3, pred.IsEqualTo(data))
}

func TestUnsubscribe(t *testing.T) {
	assert := testpredicate.NewAsserter(t)

	ch := multicast.NewChan()
	s1 := ch.SubscribeBuffered(10)
	s2 := ch.SubscribeBuffered(10)
	s3 := ch.SubscribeBuffered(10)

	data1 := makeData(0, 5)
	feedChan(data1, ch.In())
	s1.Close()
	s2.Close()

	data2 := makeData(5, 5)
	feedChan(data2, ch.In())
	ch.Close()
	r1 := drainChan(s1.Out())
	r2 := drainChan(s2.Out())
	r3 := drainChan(s3.Out())

	// Due to weak synchronization, r1 and r2 might be short by at most one
	// element, since the input is unbuffered, and they must match the beginning
	// of the original data
	assert.That(r1, pred.Length(pred.Ge(4)))
	assert.That(data1, pred.StartsWith(r1))

	assert.That(r2, pred.Length(pred.Ge(4)))
	assert.That(data1, pred.StartsWith(r2))

	assert.That(r3, pred.StartsWith(data1))
	assert.That(r3, pred.EndsWith(data2))
	assert.That(r3, pred.Length(pred.IsEqualTo(10)))
}

func TestSubscriptionOnClosedChannel(t *testing.T) {
	assert := testpredicate.NewAsserter(t)

	ch := multicast.NewChan()
	ch.Close()

	time.Sleep(10 * time.Millisecond)

	s := ch.Subscribe()
	r := drainChan(s.Out())

	assert.That(r, pred.IsEmpty())
}

func TestCloseSubscriptionOnLoadedChannel(t *testing.T) {
	assert := testpredicate.NewAsserter(t)

	ch := multicast.NewChan()
	var stop AtomicBool

	go func() {
		for i := 0; i < 100000; i++ {
			ch.In() <- i
			if stop.Get() {
				break
			}
		}
		ch.Close()
	}()

	s := ch.Subscribe()
	r := drainChanN(s.Out(), 100)
	s.Close()

	time.Sleep(1 * time.Millisecond)
	stop.Set(true)

	assert.That(r, pred.Length(pred.IsEqualTo(100)))
}

func TestCloseMulticastChanWithActiveSubscriptions(t *testing.T) {
	ch := multicast.NewChan()
	var stop AtomicBool
	go func() {
		for i := 0; i < 1000000; i++ {
			ch.In() <- i
			if stop.Get() {
				break
			}
		}
		ch.Close()
	}()

	count := 100
	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	cond := sync.NewCond(&mu)

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			s := ch.Subscribe()
			drainChanN(s.Out(), 100)

			mu.Lock()
			cond.Signal()
			mu.Unlock()

			s.Close()
			wg.Done()
		}()
	}
	mu.Lock()
	cond.Wait() // Wait for the first subscription to be full
	mu.Unlock()

	stop.Set(true)
	wg.Wait()
}

func TestCloseMulticasteChanWithActiveSubscriptions(t *testing.T) {
	ch := multicast.NewChan()
	var stop AtomicBool
	go func() {
		for i := 0; i < 1000000; i++ {
			ch.In() <- i
			if stop.Get() {
				break
			}
		}
		ch.Close()
	}()

	wg := sync.WaitGroup{}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			s := ch.Subscribe()
			<-s.Out()
			s.Close()
			wg.Done()
		}()
	}

	wg.Wait()

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			s := ch.Subscribe()
			<-s.Out()
			s.Close()
			wg.Done()
		}()
	}

	stop.Set(true)
	wg.Wait()
}
