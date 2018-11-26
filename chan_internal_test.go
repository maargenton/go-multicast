package multicast

import (
	"testing"

	testpredicate "github.com/marcus999/go-testpredicate"
	"github.com/marcus999/go-testpredicate/pred"
)

func TestNewSubscriptionOnNilChannelIsClosedImmediately(t *testing.T) {
	assert := testpredicate.NewAsserter(t)

	s := newSunscription(nil, 10)
	_, ok := <-s.Out()
	assert.That(ok, pred.IsEqualTo(false), "s.Out() should be closed")
}

func TestClosedSubscriptionsAreSafeToCLoseAgain(t *testing.T) {
	s := newSunscription(nil, 10)
	for i := 0; i < 10; i++ {
		s.Close()
	}
}

func TestClosedChannelCanBeClosedAgain(t *testing.T) {
	ch := newChan(0)
	ch.Close()

	for i := 0; i < 10; i++ {
		ch.Close()
	}
}

func TestSubscriptionsCanBeClosedAfterChannelIsClosed(t *testing.T) {
	ch := newChan(0)
	ch.Close()
	s := newSunscription(ch, 10)
	s.Close()
}
