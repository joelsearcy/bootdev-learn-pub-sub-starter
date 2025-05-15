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

	pauseCh, pauseQueue, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, routing.PauseKey+"."+username, routing.PauseKey, pubsub.SimpleQueueTypeTransient)
	if err != nil {
		fmt.Println("Failed to declare and bind pause queue:", err)
		return
	}
	fmt.Printf("Successfully declared and bound pause queue: %s...\n", pauseQueue.Name)
	defer pauseCh.Close()

	moveCh, moveQueue, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, routing.ArmyMovesPrefix+"."+username, routing.ArmyMovesPrefix+".*", pubsub.SimpleQueueTypeTransient)
	if err != nil {
		fmt.Println("Failed to declare and bind army move queue:", err)
		return
	}
	fmt.Printf("Successfully declared and bound ary move queue: %s...\n", moveQueue.Name)
	defer moveCh.Close()

	warCh, warQueue, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, routing.WarRecognitionsPrefix+".*", pubsub.SimpleQueueTypeDurable)
	if err != nil {
		fmt.Println("Failed to declare and bind war queue:", err)
		return
	}
	fmt.Printf("Successfully declared and bound war queue: %s...\n", warQueue.Name)
	defer warCh.Close()

	gameState := gamelogic.NewGameState(username)
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, routing.PauseKey+"."+username, routing.PauseKey, pubsub.SimpleQueueTypeTransient, handlerPause(gameState))
	if err != nil {
		fmt.Println("Failed to subscribe to pause queue:", err)
		return
	}
	fmt.Println("Successfully subscribed to pause queue...")

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+username, routing.ArmyMovesPrefix+".*", pubsub.SimpleQueueTypeTransient, handlerMove(gameState, warCh))
	if err != nil {
		fmt.Println("Failed to subscribe to army moves queue:", err)
		return
	}
	fmt.Println("Successfully subscribed to army moves queue...")

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, routing.WarRecognitionsPrefix+".*", pubsub.SimpleQueueTypeDurable, handlerWar(gameState))
	if err != nil {
		fmt.Println("Failed to subscribe to war queue:", err)
		return
	}
	fmt.Println("Successfully subscribed to war queue...")

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
			armyMove, err := gameState.CommandMove(input)
			if err == nil {
				fmt.Println("Move successful!")
				pubsub.PublishJSON(moveCh, routing.ExchangePerilTopic, moveQueue.Name, armyMove)
				fmt.Println("Move successfully published!")
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

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(state routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(state)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, warCh *amqp.Channel) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		moveOutcome := gs.HandleMove(move)
		switch moveOutcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			routingKey := fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, move.Player.Username)
			war := gamelogic.RecognitionOfWar{
				Attacker: move.Player,
				Defender: gs.Player,
			}
			err := pubsub.PublishJSON(warCh, routing.ExchangePerilTopic, routingKey, war)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(war gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		warOutcome, _, _ := gs.HandleWar(war)
		switch warOutcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			fallthrough
		case gamelogic.WarOutcomeYouWon:
			fallthrough
		case gamelogic.WarOutcomeDraw:
			return pubsub.Ack
		default:
			fmt.Println("Unknown war outcome")
			return pubsub.NackDiscard
		}
	}
}
