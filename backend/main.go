package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/antoniodipinto/ikisocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// Basic chat message object
type RequestMessageObject struct {
	Data string `json:"data"`
	From string `json:"from"`
	To   string `json:"to"`
}

type ResponseMessageObject struct {
	Data string `json:"data"`
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

func main() {

	// The key for the map is message.to
	clients := make(map[string]string)

	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World 👋!")
	})

	app.Use(func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// Multiple event handling supported
	ikisocket.On(ikisocket.EventConnect, func(ep *ikisocket.EventPayload) {
		fmt.Println(fmt.Sprintf("Connection event 1 - User: %s", ep.SocketAttributes["user_id"]))
	})

	// On message event
	ikisocket.On(ikisocket.EventMessage, func(ep *ikisocket.EventPayload) {

		fmt.Println(fmt.Sprintf("Message event - User: %s - Message: %s", ep.SocketAttributes["user_id"], string(ep.Data)))

		message := RequestMessageObject{}

		// Unmarshal the json message
		// {
		//  "from": "<user-id>",
		//  "to": "<recipient-user-id>",
		//  "data": "hello"
		//}
		err := json.Unmarshal(ep.Data, &message)
		if err != nil {
			fmt.Println(err)
			return
		}
		response := ResponseMessageObject{
			Data: message.Data,
			From: message.From,
			To:   message.To,
			Type: "message",
		}
		data, err := json.Marshal(response)
		if err != nil {
			fmt.Println(err)
			return
		}

		// Emit the message directly to specified user
		err = ep.Kws.EmitTo(clients[message.To], data)
		if err != nil {
			fmt.Println(err)
		}
	})

	// On disconnect event
	ikisocket.On(ikisocket.EventDisconnect, func(ep *ikisocket.EventPayload) {
		// Remove the user from the local clients
		delete(clients, ep.SocketAttributes["user_id"])
		fmt.Println(fmt.Sprintf("Disconnection event - User: %s", ep.SocketAttributes["user_id"]))
	})

	// On close event
	// This event is called when the server disconnects the user actively with .Close() method
	ikisocket.On(ikisocket.EventClose, func(ep *ikisocket.EventPayload) {
		// Remove the user from the local clients
		delete(clients, ep.SocketAttributes["user_id"])
		fmt.Println(fmt.Sprintf("Close event - User: %s", ep.SocketAttributes["user_id"]))
	})

	// On error event
	ikisocket.On(ikisocket.EventError, func(ep *ikisocket.EventPayload) {
		if ep.Error.Error() == "websocket: close 1001 (going away)" {
			fmt.Println("everything is ok")
			return
		}

		fmt.Println(ep.Error.Error())
		fmt.Println(fmt.Sprintf("Error event - User: %s", ep.SocketAttributes["user_id"]))
	})

	app.Get("/ws/:id", ikisocket.New(func(kws *ikisocket.Websocket) {

		// Retrieve the user id from endpoint
		userID := kws.Params("id")

		// Add the connection to the list of the connected clients
		// The UUID is generated randomly and is the key that allow
		// ikisocket to manage Emit/EmitTo/Broadcast
		clients[userID] = kws.UUID

		// Every websocket connection has an optional session key => value storage
		kws.SetAttribute("user_id", userID)

		//Broadcast to all the connected users the newcomer
		kws.Broadcast([]byte(fmt.Sprintf(`{"type": "notify", "data": "New user connected: %s and UUID: %s"}`, userID, kws.UUID)), true)
		//Write welcome message

		kws.Emit([]byte(fmt.Sprintf(`{"type": "notify", "data": "Hello user: %s with UUID: %s"}`, userID, kws.UUID)))
	}))

	app.Listen(":" + os.Getenv("PORT"))
}
