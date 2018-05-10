package main

import (
	"net/url"
	"flag"
	"github.com/gorilla/websocket"
	"encoding/json"
	"fmt"
	"log"
	"io"
	"bytes"
	"os"
	"os/signal"
	"time"
)

var addr = flag.String("addr", "18.217.77.98:80", "server address")
var numClients = flag.Int("clients", 100, "number of clients")

const (
	pingPeriod = 5 * time.Second

	// Shows how fast should the clients be spawned.
	clientSpawnTime = 50 * time.Millisecond
)

func main() {
	flag.Parse()
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	fmt.Println("Connecting to: " + u.String())

	for i := 0; i < *numClients; i++ {
		time.Sleep(clientSpawnTime)
		go func(j int) {
			startClient(u.String(), j)
		}(i)
	}

	waitForInterrupt()
}

func startClient(url string, clientID int) {
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)

	if err != nil {
		if err == websocket.ErrBadHandshake {
			buf := make([]byte, 1024)
			io.ReadFull(resp.Body, buf)
			index := bytes.IndexByte(buf, 0)

			log.Print("ERROR CODE ", resp.StatusCode,
				" UPGRADE ", resp.Header.Get("Upgrade"),
				" CONNECTION ", resp.Header.Get("Connection"),
				" Webscoket Accept", resp.Header.Get("Sec-Websocket-Accept"),
				" BODY ", string(buf[:index]))
		}
		log.Print("ERROR DIALING " + err.Error())
	} else {
		client := Client{Conn: conn, ID: clientID}
		go sendMessage(&client)
		go readChannel(&client)
	}
}

func sendMessage(client *Client) {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
			case <-ticker.C:
				message, _ := json.Marshal(Message{Body: fmt.Sprintf("HERE I AM %d", client.ID),ClientID: client.ID})
				err := client.Conn.WriteMessage(websocket.TextMessage, message)

				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseGoingAway) {
						log.Print("Client closed connection: "+ err.Error())
					}
					log.Print("Could not write message"+err.Error())
					return
				}
		}
	}
}

func readChannel(client *Client) {
	defer func() {
		client.Conn.Close()
		log.Println("CLOSING CLIENT ", client.ID)
	}()
	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseGoingAway) {
				log.Print("Client closed connection: "+ err.Error())
			}
			break
		}
		var incoming Message
		json.Unmarshal(message, &incoming)

		//log.Print("MESSAGE RECEIVED: ", incoming.Body)
	}
}

func waitForInterrupt() {
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for range signalChan {
			log.Println("\nReceived an interrupt, stopping services...\n")
			cleanupDone <- true
		}
	}()
	<-cleanupDone
}