package socket

import (
	"fmt"
	"log"
	"net/http"

	socketio "github.com/googollee/go-socket.io"
)

func SocketI() {
	server := socketio.NewServer(nil)

	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		fmt.Println("connected:", s.ID())
		return nil
	})

	server.OnEvent("/items", "join", func(s socketio.Conn, msg string) {
		s.Join(msg)
	})

	server.OnEvent("/items", "events", func(s socketio.Conn, msg string) {
		server.BroadcastToRoom("/items", "123", "new-json", msg)
	})

	server.OnEvent("/items", "bye", func(s socketio.Conn, msg string) string {
		last := s.Context().(string)
		s.Leave(msg)
		s.Emit("bye", last)
		s.Close()
		return last
	})

	server.OnError("/item", func(s socketio.Conn, e error) {
		// server.Remove(s.ID())
		fmt.Println("meet error:", e)
	})

	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		// Add the Remove session id. Fixed the connection & mem leak
		fmt.Println("closed", reason)
	})

	go server.Serve()
	defer server.Close()

	http.Handle("/socket.io/", server)
	log.Println("Serving at localhost:8001...")
	log.Fatal(http.ListenAndServe(":8001", nil))
}
