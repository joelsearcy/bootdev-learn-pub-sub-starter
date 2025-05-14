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
	fmt.Println("Starting Peril client...")
	conn, err := amqp.Dial(conn_str)
	if err != nil {
		fmt.Println("Failed to connect to RabbitMQ:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Successfully connected to RabbitMQ broker...")

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Println("Failed to get username:", err)
		return
	}
	queueName := fmt.Sprintf("%s.%s", routing.PauseKey, username)
	ch, queue, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, queueName, routing.PauseKey, pubsub.SimpleQueueTypeTransient)
	if err != nil {
		fmt.Println("Failed to declare and bind queue:", err)
		return
	}
	fmt.Printf("Successfully declared and bound queue: %s...\n", queue.Name)
	defer ch.Close()

	gameState := gamelogic.NewGameState(username)
	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}
		err = nil

		cmd := strings.ToLower(input[0])
		switch cmd {
		case "spawn":
			err = gameState.CommandSpawn(input)
		case "move":
			_, err := gameState.CommandMove(input)
			if err == nil {
				fmt.Println("Move successful!")
			}
		case "status":
			gameState.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			return
		default:
			fmt.Println("Unknown command. Please try again.")
		}
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}
	}

	// signalChan := make(chan os.Signal, 1)
	// signal.Notify(signalChan, os.Interrupt)
	// <-signalChan
	// fmt.Println("Shutting down...")
}
