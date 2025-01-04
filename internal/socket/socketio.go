package socket

import (
	"encoding/json"
	"fmt"
	"github.com/zishang520/socket.io/v2/socket"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

type notification struct {
	Data  string      `json:"data"`
	Room  socket.Room `json:"room"`
	Event string      `json:"event"`
}

func SocketI() {
	io := socket.NewServer(nil, nil)
	http.Handle("/socket.io/", io.ServeHandler(nil))
	go func() {
		err := http.ListenAndServe(":8001", nil)
		if err != nil {
			fmt.Println(err.Error())
		}
	}()

	io.Of("/items", nil).On("connection", func(clients ...any) {
		client := clients[0].(*socket.Socket)
		client.On("events", func(datas ...any) {
			var events notification
			str := fmt.Sprintf("%v", datas[0])
			data := []byte(str)
			err := json.Unmarshal(data, &events)
			if err != nil {
				log.Println(err.Error())
			}
			client.Broadcast().To(events.Room).Emit(events.Event, events.Data)
		})
		client.On("disconnect", func(data ...any) {
			log.Println("closed", data)
		})

		client.On("join", func(data ...any) {
			channel := socket.Room(fmt.Sprintf("%v", data[0]))
			client.Join(channel)
		})
	})

	exit := make(chan struct{})
	SignalC := make(chan os.Signal)

	signal.Notify(SignalC, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range SignalC {
			switch s {
			case os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				close(exit)
				return
			}
		}
	}()

	<-exit
	io.Close(nil)
	os.Exit(0)
}
