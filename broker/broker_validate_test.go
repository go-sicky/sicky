package broker_test

import (
	"testing"

	brkJetstream "github.com/go-sicky/sicky/broker/jetstream"
	brkNats "github.com/go-sicky/sicky/broker/nats"
	brkNsq "github.com/go-sicky/sicky/broker/nsq"
)

func TestBrokerNewRejectsInvalidConfig(t *testing.T) {
	if brk := brkNsq.New(nil, &brkNsq.Config{MaxInFlight: -1}); brk != nil {
		t.Fatal("nsq New with negative max_in_flight should return nil")
	}

	if brk := brkNsq.New(nil, &brkNsq.Config{Compression: "bogus"}); brk != nil {
		t.Fatal("nsq New with unknown compression should return nil")
	}

	if brk := brkJetstream.New(nil, &brkJetstream.Config{
		Stream: &brkJetstream.StreamConfig{MaxConsumers: -1},
	}); brk != nil {
		t.Fatal("jetstream New with negative max_consumers should return nil")
	}

	if brk := brkNats.New(nil, &brkNats.Config{}); brk == nil {
		t.Fatal("nats New with empty config should fill defaults, not nil")
	}
}
