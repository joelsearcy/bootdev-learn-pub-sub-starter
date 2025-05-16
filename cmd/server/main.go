package main

import (
	"fmt"
	"strings"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	conn_str := "amqp://guest:guest@localhost:5672/"
	fmt.Println("Starting Peril server...")
	conn, err := amqp.Dial(conn_str)
	if err != nil {
		fmt.Println("Failed to connect to RabbitMQ:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Successfully connected to RabbitMQ broker...")

	ch, err := conn.Channel()
	if err != nil {
		fmt.Println("Failed to open a channel:", err)
		return
	}
	defer ch.Close()
	fmt.Println("Successfully opened a channel...")

	// logCh, logQueue, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, routing.GameLogSlug, routing.GameLogSlug+".*", pubsub.SimpleQueueTypeDurable)
	// if err != nil {
	// 	fmt.Println("Failed to declare and bind game logs queue:", err)
	// 	return
	// }
	// defer logCh.Close()
	// fmt.Printf("Successfully created game logs queue: %s...\n", logQueue.Name)

	err = pubsub.SubscribeGOB(conn, routing.ExchangePerilTopic, routing.GameLogSlug, routing.GameLogSlug+".*", pubsub.SimpleQueueTypeDurable, handlerGameLogs())
	if err != nil {
		fmt.Println("Failed to subscribe to game logs queue:", err)
		return
	}
	fmt.Println("Successfully subscribed to game logs queue...")

	gamelogic.PrintServerHelp()

	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		cmd := strings.ToLower(input[0])
		switch cmd {
		case "help":
			gamelogic.PrintServerHelp()
		case "pause":
			fmt.Println("Pausing game...")
			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
			if err != nil {
				fmt.Println("Failed to publish message:", err)
				return
			}
		case "resume":
			fmt.Println("Resuming game...")
			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false})
			if err != nil {
				fmt.Println("Failed to publish message:", err)
				return
			}
		case "quit":
			fmt.Println("Exiting game...")
			return
		default:
			fmt.Println("Unknown command. Please try again.")
		}
	}

	// signalChan := make(chan os.Signal, 1)
	// signal.Notify(signalChan, os.Interrupt)
	// <-signalChan
	// fmt.Println("Shutting down...")
}

func handlerGameLogs() func(routing.GameLog) pubsub.AckType {
	return func(gl routing.GameLog) pubsub.AckType {
		defer fmt.Print("> ")
		err := gamelogic.WriteLog(gl)
		if err != nil {
			fmt.Println("Failed to write game log:", err)
			return pubsub.NackRequeue
		}
		return pubsub.Ack
	}
}
