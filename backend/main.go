package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/antoniodipinto/ikisocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/mediocregopher/radix/v3"
	"github.com/vmihailenco/msgpack/v5"
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

type Item struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func main() {

	// The key for the map is message.to
	clients := make(map[string]string)

	app := fiber.New()

	redisRepo := createRedis()
	// redisRepo.UpdateData()

	go createPubsub()

	app.Get("/publish", func(c *fiber.Ctx) error {
		i := []Item{
			{Name: "Alice", Age: 1},
			{Name: "Bas", Age: 18},
		}

		if err := redisRepo.Publish(i); err != nil {
			return c.SendString("error")
		}
		return c.SendString("published")
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World 👋!")
	})

	// type RequestData struct {

	// }

	app.Post("/data", func(c *fiber.Ctx) error {
		type ReqBody struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		var body ReqBody
		if err := c.BodyParser(&body); err != nil {
			c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{
				"message": err,
			})
			return err
		}
		fmt.Println(body)
		redisRepo.UpdateUserData(1, body)
		return c.JSON(&fiber.Map{
			"message": "wo",
		})
	})

	app.Get("/data/:id", func(c *fiber.Ctx) error {
		ID, _ := strconv.Atoi(c.Params("id"))
		data, _ := redisRepo.GetUserData(ID)
		return c.JSON(data)
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

type RedisRepo struct {
	rds *radix.Pool
}

func createRedis() *RedisRepo {
	pool, err := radix.NewPool("tcp", "127.0.0.1:6379", 10)
	if err != nil {
		// handle error
		fmt.Println("Handle error", err.Error())
	}

	return &RedisRepo{rds: pool}
}

// UpdateData update data on redis
func (r RedisRepo) Publish(data interface{}) error {
	b, err := msgpack.Marshal(data)
	if err != nil {
		return err
	}

	if err = r.rds.Do(radix.Cmd(nil, "PUBLISH", "myChannel", string(b))); err != nil {
		return err
	}

	return nil
}

// UpdateData update data on redis
func (r RedisRepo) UpdateUserData(userID int, data interface{}) error {
	b, err := msgpack.Marshal(data)
	if err != nil {
		return err
	}

	return r.rds.Do(radix.FlatCmd(nil, "HSET", "u:data", fmt.Sprintf("%d", userID), b))
}

func (r RedisRepo) GetUserData(userID int) (interface{}, error) {
	var data []byte
	// b, err := msgpack.Marshal(data)
	if err := r.rds.Do(radix.Cmd(&data, "HGET", "u:data", strconv.Itoa(userID))); err != nil {
		return nil, err
	}

	var result interface{}

	if err := msgpack.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("data not found")
	}

	return &result, nil
}

func createPubsub() radix.PubSubConn {
	fmt.Println("create pubsub")
	// Create a normal redis connection
	conn, err := radix.Dial("tcp", "127.0.0.1:6379")
	if err != nil {
		panic(err)
	}

	ps := radix.PubSub(conn)
	defer ps.Close()

	msgCh := make(chan radix.PubSubMessage)
	if err := ps.Subscribe(msgCh, "myChannel"); err != nil {
		panic(err)
	}
	// It's optional, but generally advisable, to periodically Ping the
	// connection to ensure it's still alive. This should be done in a separate
	// go-routine from that which is reading from msgCh.
	errCh := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := ps.Ping(); err != nil {
				errCh <- err
				return
			}
		}
	}()

	for {
		select {
		case msg := <-msgCh:
			log.Printf("publish to channel %q received: %q", msg.Channel, msg.Message)
			decodeMessage(msg.Message)
		case err := <-errCh:
			panic(err)
		}
	}
}

func decodeMessage(data []byte) {
	var i []Item
	if err := msgpack.Unmarshal(data, &i); err != nil {
		fmt.Println("log error")
	}

	fmt.Println(i)
	fmt.Println(i[0])
}
