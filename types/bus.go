package types

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type MessageBus struct {
	redisClient *redis.Client
	channel     string
}

func NewMessageBus(redisClient *redis.Client, channel string) *MessageBus {
	return &MessageBus{
		redisClient: redisClient,
		channel:     channel,
	}
}

func (bus *MessageBus) Publish(msg Message) error {
	bz, err := MarshalMessage(msg)
	if err != nil {
		return err
	}
	if err := bus.redisClient.Publish(context.Background(), bus.channel, bz).Err(); err != nil {
		return err
	}
	return nil
}

func (bus *MessageBus) Subscribe() (ch <-chan Message, closeCh func()) {
	ps := bus.redisClient.Subscribe(context.Background(), bus.channel)
	psCh := ps.Channel()
	msgCh := make(chan Message, 1)
	go func() {
		defer close(msgCh)
		for msg := range psCh {
			msg, err := UnmarshalMessage([]byte(msg.Payload))
			if err != nil {
				panic(msg)
			}
			msgCh <- msg
		}
	}()
	return msgCh, func() { ps.Close() }
}
