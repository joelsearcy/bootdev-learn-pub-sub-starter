package pubsub

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int

const (
	SimpleQueueTypeDurable SimpleQueueType = iota + 1
	SimpleQueueTypeTransient
)

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	durable := false
	autoDelete := false
	exclusive := false
	if simpleQueueType == SimpleQueueTypeDurable {
		durable = true
	}
	if simpleQueueType == SimpleQueueTypeTransient {
		autoDelete = true
		exclusive = true
	}

	ampqTable := amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	}
	queue, err := ch.QueueDeclare(queueName, durable, autoDelete, exclusive, false, ampqTable)
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	err = ch.QueueBind(queue.Name, key, exchange, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	return ch, queue, nil
}
