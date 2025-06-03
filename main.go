package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

type webSocketHandler struct {
	upgrader websocket.Upgrader
}

type ActionMessage struct {
	Action  string `json:"action"`
	Content string `json:"content"`
	User_id int    `json:"user_id,omitempty"`
}

type Connection struct {
	Username string
	c        *websocket.Conn
}

type Room struct {
	Name  string
	Users map[int]*Connection
}

var room = Room{
	Name:  "default",
	Users: make(map[int]*Connection),
}

func sendErrorMessage(c *websocket.Conn, error_msg string) {
	// Send an error message to the WebSocket client
	marshalledError, _ := json.Marshal(&ActionMessage{
		Action:  "error",
		Content: error_msg,
	})

	err := c.WriteMessage(websocket.TextMessage, marshalledError)

	if err != nil {
		log.Printf("error %s when writing error message to client", err)
	}
}

func sendJSONMessage(c *websocket.Conn, message []byte) {
	err := c.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		log.Printf("error %s when writing JSON message to client", err)
	}
}

// returns a list of integers for a given dice string
// "2d4" returns [2, 1], a list of length 2 with values between 1 and 4
func rollDice(dice string) ([]int, error) {
	die := strings.Split(dice, "d")

	n, err := strconv.Atoi(die[0])
	if err != nil {
		log.Printf("error %s when parsing n_die", err)
		return nil, err
	}

	value, err := strconv.Atoi(die[1])
	if err != nil {
		log.Printf("error %s when parsing value_die", err)
		return nil, err
	}

	total := make([]int, n)

	for i := range n {
		total[i] = rand.Intn(value) + 1
	}

	return total, nil
}

func handleActionMessage(c *websocket.Conn, msg ActionMessage) {
	// Handle the action message based on its content
	log.Printf("Handling action: %s with content: %s", msg.Action, msg.Content)
	var err error
	switch msg.Action {
	case "roll":
		result := map[string][]int{}
		dice := strings.Split(msg.Content, "+")

		for i := range dice {
			result[dice[i]], err = rollDice(dice[i])

			if err != nil {
				sendErrorMessage(c, "Error rolling dice")
				return
			}
		}

		marshalledResult, _ := json.Marshal(result)
		marshalledResponse, _ := json.Marshal(&ActionMessage{"roll", string(marshalledResult)})

		log.Printf("Sending response: %s", string(marshalledResponse))
		sendJSONMessage(c, marshalledResponse)
	case "log":
		// Create new user id

	}
}

func (wsh webSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	c, err := wsh.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("error %s when upgrading connection", err)
		return
	}
	// Close the WebSocket connection when serving is done
	defer c.Close()

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Printf("error %s when reading message from client", err)
			return
		}

		if mt == websocket.BinaryMessage {
			sendErrorMessage(c, "Binary message received")
			return
		}

		if mt == websocket.TextMessage {
			log.Printf("Received message: %s", message)
			// Parse the recieved JSON
			var parsedMessage ActionMessage
			err = json.Unmarshal(message, &parsedMessage)

			if err != nil {
				sendErrorMessage(c, "Error parsing message")
				continue
			}

			handleActionMessage(c, parsedMessage)
		}

	}
}

func main() {
	// Define WebSocket handler
	webSocketHandler := webSocketHandler{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == "http://127.0.0.1:5500"
			},
		},
	}

	http.Handle("/", webSocketHandler)
	log.Print("Start server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
