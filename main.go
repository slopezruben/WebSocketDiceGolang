package main

import (
	"encoding/json"
	"log"
	"maps"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	Username string          `json:"username"`
	c        *websocket.Conn `json:"-"`
}

type Room struct {
	Name  string
	Users map[int]*Connection
}

var room = Room{
	Name:  "default",
	Users: make(map[int]*Connection),
}

func addUserToRoom(room_name string, userId int, userConnection *Connection) {
	// Add a user to the room
	if _, exists := room.Users[userId]; exists {
		log.Printf("User with ID %d already exists in room %s", userId, room)
		return
	}

	room.Users[userId] = userConnection
	log.Printf("User %s with ID %d added to room %s", userConnection.Username, userId, room)
}

func removeUserFromRoom(room_name string, userId int) {
	// Remove a user from the room
	if _, exists := room.Users[userId]; !exists {
		log.Printf("User with ID %d does not exist in room %v", userId, room)
		return
	}
	delete(room.Users, userId)
	log.Printf("User with ID %d removed from room %v", userId, room)
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

func sendMessageToRoom(room_name string, message []byte) {
	for userConnection := range maps.Values(room.Users) {
		sendJSONMessage(userConnection.c, message)
		log.Printf("Sent message to user %s in room %s", userConnection.Username, room_name)
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

func handleActionMessage(c *websocket.Conn, msg ActionMessage, userId int) {
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
		marshalledResponse, _ := json.Marshal(&ActionMessage{"roll", string(marshalledResult), userId})

		sendMessageToRoom("", marshalledResponse)
	case "log":
		userConnection := &Connection{
			Username: msg.Content,
			c:        c,
		}

		marshalledResponse, _ := json.Marshal(&ActionMessage{
			Action:  "log",
			Content: strconv.Itoa(userId),
		})
		sendJSONMessage(c, marshalledResponse)

		// Ejemplo para construir el objeto de usuarios:
		users := make(map[int]map[string]any)
		for id, conn := range room.Users {
			users[id] = map[string]any{
				"user_id":  id,
				"username": conn.Username,
			}
		}
		users_in_room, _ := json.Marshal(users)
		if len(room.Users) > 0 {
			// Inform the user of the other users in the room
			marshalledResponse, _ = json.Marshal(&ActionMessage{
				Action:  "users_in_room",
				Content: string(users_in_room),
				User_id: userId,
			})

			sendJSONMessage(c, marshalledResponse)
		}

		addUserToRoom("", userId, userConnection)

		marshalledResponse, _ = json.Marshal(&ActionMessage{
			Action:  "user_joined",
			Content: userConnection.Username,
			User_id: userId,
		})

		// Notify all users in the room about the new user
		sendMessageToRoom("", marshalledResponse)
	case "logout":
		log.Printf("Logging out user with ID: %d", msg.User_id)
		removeUserFromRoom("", msg.User_id)

		marshalledResponse, _ := json.Marshal(&ActionMessage{
			Action:  "user_left",
			Content: "",
			User_id: msg.User_id,
		})

		// Notify all users in the room about the new user
		sendMessageToRoom("", marshalledResponse)
	default:
		log.Printf("Unknown action: %s", msg.Action)
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

	// Create new user id from time
	userId := int(time.Now().UnixNano())

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Printf("error %s when reading message from client", err)
			if userId != 0 {
				removeUserFromRoom("", userId)
				marshalledResponse, _ := json.Marshal(&ActionMessage{
					Action:  "user_left",
					Content: "",
					User_id: userId,
				})
				sendMessageToRoom("", marshalledResponse)
			}
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

			handleActionMessage(c, parsedMessage, userId)
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
