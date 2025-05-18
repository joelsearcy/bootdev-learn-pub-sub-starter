package pubsub

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type AckType int

const (
	Ack AckType = iota
	NackDiscard
	NackRequeue
)

func subscribe[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType,
	handler func(T) AckType,
	unmarshaller func([]byte) (T, error),
) error {
	ch, queue, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return fmt.Errorf("failed to declare and bind queue: %w", err)
	}

	err = ch.Qos(10, 0, false)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	deliveryCh, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to consume messages: %w", err)
	}

	go func() {
		for msg := range deliveryCh {
			val, err := unmarshaller(msg.Body)
			if err != nil {
				fmt.Printf("failed to unmarshal message: %v\n", err)
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
