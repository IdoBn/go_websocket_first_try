package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var hubs []Hub
var identifier = 0

type Hub struct {
	id   int
	send chan interface{}
	conn *websocket.Conn
}

type Message struct {
	HubId int
	Msg   interface{}
}

func (h *Hub) Listen(messages chan interface{}) {
	for json := range messages {
		err := h.conn.WriteJSON(json)
		if err != nil {
			log.Println("something went wrong while sending message to hub")
		}
	}
}

func getWsHandler(upgrader *websocket.Upgrader, addHub chan Hub, removeHub chan Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		h := Hub{
			id:   identifier,
			send: make(chan interface{}),
			conn: conn,
		}

		identifier = identifier + 1

		addHub <- h
		h.Listen(h.send)
		defer func() { removeHub <- h }()
	}
}

func hubWorker() (chan Hub, chan Hub, chan Message) {
	addHub := make(chan Hub)
	removeHub := make(chan Hub)
	findHub := make(chan Message)
	go func() {
		for {
			select {
			case h := <-addHub:
				hubs = append(hubs, h)
			case h := <-removeHub:
				for i, hub := range hubs {
					if hub.id == h.id {
						hubs = append(hubs[:i], hubs[i+1:]...)
						return
					}
				}
			case m := <-findHub:
				hubs[m.HubId].send <- m.Msg
			}
		}
	}()
	return addHub, removeHub, findHub
}

func main() {
	port := 8089

	addHub, removeHub, findHub := hubWorker()

	upgrader := &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}
	http.Handle("/ws", getWsHandler(upgrader, addHub, removeHub))

	fs := http.Dir("./")
	fileHandler := http.FileServer(fs)
	http.Handle("/", fileHandler)

	http.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		findHub <- Message{0, `{"msg": "i am json fear me!!!"}`}
	})

	log.Printf("Running on port %d\n", port)
	addr := fmt.Sprintf(":%d", port)

	err := http.ListenAndServe(addr, nil)
	fmt.Println(err.Error())
}
