package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sort"

	"github.com/gorilla/websocket"
)

var wsChan = make(chan WsPayload)
var clients = make(map[WebSocketConnection]string)

var upgradeConnection = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type WebSocketConnection struct {
	*websocket.Conn
}

// WsJsonResponse defines the response sent back from websocket
type WsJsonResponse struct {
	Action         string   `json:"action"`
	Message        string   `json:"message"`
	MessageType    string   `json:"messageType"`
	ConnectedUsers []string `json:"connectedUsers"`
}

type WsPayload struct {
	Action   string              `json:"action"`
	Username string              `json:"username"`
	Message  string              `json:"message"`
	Conn     WebSocketConnection `json:"-"`
}

// WsEndpoint upgrades connection to websocket
func WsEndpoint(w http.ResponseWriter, r *http.Request) {
	ws, err := upgradeConnection.Upgrade(w, r, nil)
	if err != nil {
		log.Panicln(err)
	}

	log.Println("Client connected to endpoint")

	var res WsJsonResponse

	res.Message = `<em><small>Connected to server</small></em>`

	conn := WebSocketConnection{Conn: ws}

	clients[conn] = ""

	err = ws.WriteJSON(res)
	if err != nil {
		log.Println(err)
	}

	go ListenForWs(&conn)
}

func ListenForWs(conn *WebSocketConnection) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Error", fmt.Sprintf("%v", r))
		}

		var payload WsPayload

		for {
			err := conn.ReadJSON(&payload)
			if err != nil {
				log.Println(err, payload)
				return
			}

			payload.Conn = *conn
			wsChan <- payload
		}

	}()
}

func ListenToWsChannel() {
	var res WsJsonResponse

	for {
		e := <-wsChan

		switch e.Action {
		case "username":
			clients[e.Conn] = e.Username
			users := getUserList()
			res.Action = "listUsers"
			res.ConnectedUsers = users
			broadcastToAll(res)

		case "left":
			delete(clients, e.Conn)
			res.Action = "listUsers"
			users := getUserList()
			res.ConnectedUsers = users
			broadcastToAll(res)

		case "broadcast":
			res.Action = "broadcast"
			res.Message = fmt.Sprintf("<strong>%s</strong>: %s", e.Username, e.Message)
			broadcastToAll(res)
		}

	}
}

func getUserList() []string {
	var userList []string

	for _, x := range clients {
		if x == "" {
			continue
		}

		userList = append(userList, x)
	}

	sort.Strings(userList)

	return userList
}

func broadcastToAll(res WsJsonResponse) {
	for client := range clients {
		err := client.WriteJSON(res)
		if err != nil {
			log.Println("websocket error", err)
			_ = client.Close()
			delete(clients, client)
		}
	}
}
