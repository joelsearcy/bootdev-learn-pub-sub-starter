package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type AckType int

const (
	Ack AckType = iota + 1
	NackRequeue
	NackDiscard
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	// Marshal the value to JSON
	body, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Publish the message
	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType, // an enum to represent "durable" or "transient"
	handler func(T) AckType,
) error {
	ch, queue, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return fmt.Errorf("failed to declare and bind queue: %w", err)
	}

	deliveryCh, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to consume messages: %w", err)
	}

	go func() {
		for msg := range deliveryCh {
			var val T
			if err := json.Unmarshal(msg.Body, &val); err != nil {
				fmt.Printf("failed to unmarshal JSON: %v\n", err)
				continue
			}
			ackType := handler(val)
			switch ackType {
			case Ack:
				fmt.Printf("Ack-ing message: %s\n", msg.RoutingKey)
				msg.Ack(false)
			case NackRequeue:
				fmt.Printf("Nack-ing and requeue message: %s\n", msg.RoutingKey)
				msg.Nack(false, true)
			case NackDiscard:
				fmt.Printf("Nack-ing and discard message: %s\n", msg.RoutingKey)
				msg.Nack(false, false)
			default:
				fmt.Printf("unknown ack type: %v\n", ackType)
				msg.Nack(false, false)
			}
		}

	}()
	return nil
}
