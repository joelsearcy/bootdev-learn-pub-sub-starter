package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishGOB[T any](ch *amqp.Channel, exchange, key string, val T) error {
	// Encode the value with GOB
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(val)
	if err != nil {
		return fmt.Errorf("failed to encode with GOB: %w", err)
	}
	body := buffer.Bytes()
	// Publish the message
	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/gob",
		Body:        body,
	})
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

func SubscribeGOB[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType, // an enum to represent "durable" or "transient"
	handler func(T) AckType,
) error {
	unmarshaller := func(data []byte) (T, error) {
		buffer := bytes.NewBuffer(data)
		decoder := gob.NewDecoder(buffer)
		var val T
		if err := decoder.Decode(&val); err != nil {
			return val, fmt.Errorf("failed to unmarshal GOB: %w", err)
		}
		return val, nil
	}

	return subscribe(conn, exchange, queueName, key, simpleQueueType, handler, unmarshaller)
}
