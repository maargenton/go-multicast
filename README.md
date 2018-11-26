# go-multicast

Multicast channels in Go.

[![GoDoc](https://godoc.org/github.com/marcus999/go-multicast?status.svg)](https://godoc.org/github.com/marcus999/go-multicast)
[![Build Status](https://travis-ci.org/marcus999/go-multicast.svg?branch=master)](https://travis-ci.org/marcus999/go-multicast)
[![codecov](https://codecov.io/gh/marcus999/go-multicast/branch/master/graph/badge.svg)](https://codecov.io/gh/marcus999/go-multicast)
[![Go Report Card](https://goreportcard.com/badge/github.com/marcus999/go-multicast)](https://goreportcard.com/report/github.com/marcus999/go-multicast)


Package `go-multicast` implements a multicast channel for which every input value
is broadcasted to all subscribers.

With a traditional go channels and multiple concurrent readers, each value is
delivered to exaxtcly one of the readers. While this is an extremely useful
behavior, there is another common usecase where each value needs to be send to
all receivers, and that is the behavior that `multicast.Chan` implements.

Due to the lack of generic types in Go, `multicast.Chan` is implemeted as a
channel of generic `interface{}`, and therefore does not provide type safety
at compile time. It is recommecded to use type assertions at runtime to
downcast values to their intended type.
While the cost of dynamic type checking is relatively small, you might want to
generate a type specific implementation for use in performance critical code.


## Installation

    go get github.com/marcus999/go-multicast

## Usage

```go
package main

import "github.com/marcus999/go-multicast"

func producer(ch *multicast.Chan) {
    for i := 0; i < 10; i++ {
        ch.In() <- i
    }
}

func readOnce(ch *multicast.Chan) interface{} {
    s := ch.Subscribe()
    defer s.Unsubscribe

    return <-s.Out()
}

func subscribe(ch *multicast.Chan) {
    s := ch.Subscribe()
    for value := range s.Out() {
        _ = value // do somthing with value
    }
    // Reached when input channel is closed
}
```

## Details

Channels created with `multicast.NewChan()`, and subscriptions created by
`ch.Subscribe()` are unbuffered by default. To create a channel with a buffered
input channel, use `multicast.NewBufferedChan( N )`, where N is the size
of the buffer. To create a subscription with a buffered output channel, use
`ch.SubscribeBuffered( N )`, where N is the size of the buffer.

Note that the multicast channel will block if it has unattended active
subscriptions. To avoid this situation, make sure that all active subscriptions
are backed by a goroutine actively draining <-s.Out(). If the draining goroutine
is also be busy doing other things, you might want to considere adding a buffer
on the subscription.

## Design notes

At first, it might seem that implementing a multicast channel might be easy
enough that it is hardly worth using a dedicated library. Unfortunately, given
the go channel usage guidelines and restrictions, dealing reliably with closing
subscriptions is actually not trivial.

Closed channels cannot be closed again or written to, and the easiest way to
close a channel is from a single producer thread once it is done producing.
Subscriptions are read channels that need to be closed by the consumer.
There are a few interesting concurrency cases that need to be deslt with
correctly:

- Closing a multicast channel should also close all active subscriptions.

- Closing a subsccription that has already been closed should have no effect.
  This requires a mutex on the subscription for the sole purpose of synchronizing
  the closing of the channels.

- Closing a subscription from the receiving thread while the multicast runloop
  is attempting to write to the subscription channel should not result in a
  deadloack. This reauires a two-phase unsubscribe with a separate close
  channel.
