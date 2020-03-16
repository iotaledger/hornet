package monitor

// MonitorHub maintains the set of active clients and broadcasts messages to the clients.
type MonitorHub struct {
	// Registered clients.
	clients map[*MonitorClient]struct{}

	// Inbound messages from the clients.
	broadcast chan *wsMessage

	// Register requests from the clients.
	register chan *MonitorClient

	// Unregister requests from clients.
	unregister chan *MonitorClient

	shutdownSignal <-chan struct{}
}

func newHub() *MonitorHub {
	return &MonitorHub{
		clients:    make(map[*MonitorClient]struct{}),
		broadcast:  make(chan *wsMessage, BROADCAST_QUEUE_SIZE),
		register:   make(chan *MonitorClient),
		unregister: make(chan *MonitorClient),
	}
}

func (h *MonitorHub) broadcastMsg(msg *wsMessage) {
	select {
	case h.broadcast <- msg:
	default:
	}
}

func (h *MonitorHub) run(shutdownSignal <-chan struct{}) {

	for {
		select {
		case <-shutdownSignal:
			for client := range h.clients {
				delete(h.clients, client)
				close(client.sendChan)
			}
			return

		case client := <-h.register:
			// register client
			h.clients[client] = struct{}{}

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.sendChan)
				log.Infof("Removed websocket client")
			}

		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.sendChan <- message:
				default:
				}
			}
		}
	}
}
