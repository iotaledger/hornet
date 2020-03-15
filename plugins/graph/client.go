package graph

import (
	"net/http"
	"time"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 125 // 125 is the maximum payload size for ping pongs

	// Maximum size of queued messages that should be sent to the peer.
	sendChannelSize = 100
)

// GraphClient is a middleman between the node and the websocket connection.
type GraphClient struct {
	hub *GraphHub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	sendChan chan *wsMessage
}

// checkPong checks if the client is still available and answers to the ping messages
// that are sent periodically in the writePump function.
//
// At most one reader per websocket connection is allowed
func (c *GraphClient) checkPong() {

	defer func() {
		// Send a unregister message to the hub
		c.hub.unregister <- c
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warnf("Websocket error: %v", err)
			}
			return
		}
	}
}

// writePump pumps messages from the node to the websocket connection.
//
// At most one writer per websocket connection is allowed
func (c *GraphClient) writePump() {

	pingTicker := time.NewTicker(pingPeriod)

	defer func() {
		// stop the ping ticker
		pingTicker.Stop()

		// Send a unregister message to the hub
		c.hub.unregister <- c

		// close the websocket connection
		c.conn.Close()
	}()

	for {
		select {

		case <-hub.shutdownSignal:
			return

		case msg, ok := <-c.sendChan:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The GraphHub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(msg); err != nil {
				log.Warnf("Websocket error: %v", err)
				return
			}

		case <-pingTicker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *GraphClient) onConnect() {
	log.Info("WebSocket client connection established")

	config := &wsConfig{NetworkName: config.NodeConfig.GetString(config.CfgGraphNetworkName)}

	var initTxs []*wsTransaction
	txRingBufferLock.Lock()
	txRingBuffer.Do(func(tx interface{}) {
		if tx != nil {
			initTxs = append(initTxs, tx.(*wsTransaction))
		}
	})
	txRingBufferLock.Unlock()

	var initSns []*wsTransactionSn
	snRingBufferLock.Lock()
	snRingBuffer.Do(func(sn interface{}) {
		if sn != nil {
			initSns = append(initSns, sn.(*wsTransactionSn))
		}
	})
	snRingBufferLock.Unlock()

	var initMs []string
	msRingBufferLock.Lock()
	msRingBuffer.Do(func(ms interface{}) {
		if ms != nil {
			initMs = append(initMs, ms.(string))
		}
	})
	msRingBufferLock.Unlock()

	c.sendChan <- &wsMessage{Type: "config", Data: config}
	c.sendChan <- &wsMessage{Type: "inittx", Data: initTxs}
	c.sendChan <- &wsMessage{Type: "initsn", Data: initSns}
	c.sendChan <- &wsMessage{Type: "initms", Data: initMs}
}

// serveWebsocket handles websocket requests from the peer.
func serveWebsocket(hub *GraphHub, w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warnf("Upgrade websocket: %v", err)
		return
	}

	client := &GraphClient{hub: hub, conn: conn, sendChan: make(chan *wsMessage, sendChannelSize)}
	client.hub.register <- client

	go client.checkPong()
	go client.writePump()
	client.onConnect()
}
