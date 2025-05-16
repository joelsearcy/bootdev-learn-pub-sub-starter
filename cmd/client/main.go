package main

import (
	"fmt"
	"strings"
	"time"

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

	gameLogCh, gameLogQueue, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, routing.GameLogSlug, routing.GameLogSlug+".*", pubsub.SimpleQueueTypeDurable)
	if err != nil {
		fmt.Println("Failed to declare and bind war queue:", err)
		return
	}
	fmt.Printf("Successfully declared and bound war queue: %s...\n", gameLogQueue.Name)
	defer gameLogCh.Close()

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

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, routing.WarRecognitionsPrefix+".*", pubsub.SimpleQueueTypeDurable, handlerWar(gameState, gameLogCh))
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

func handlerWar(gs *gamelogic.GameState, gameLogCh *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(war gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		warOutcome, winner, loser := gs.HandleWar(war)
		logMessage := ""
		var ackResult pubsub.AckType

		switch warOutcome {
		case gamelogic.WarOutcomeNotInvolved:
			ackResult = pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			ackResult = pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			fallthrough
		case gamelogic.WarOutcomeYouWon:
			logMessage = fmt.Sprintf("%s won a war against %s", winner, loser)
			ackResult = pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			logMessage = fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
			ackResult = pubsub.Ack
		default:
			fmt.Println("Unknown war outcome")
			ackResult = pubsub.NackDiscard
		}

		if logMessage != "" {
			gameLog := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     logMessage,
				Username:    war.Attacker.Username,
			}

			err := pubsub.PublishGOB(gameLogCh, routing.ExchangePerilTopic, routing.GameLogSlug+"."+war.Attacker.Username, gameLog)
			if err != nil {
				fmt.Println("Failed to publish game log:", err)
				ackResult = pubsub.NackRequeue
			}
		}

		return ackResult
	}
}
