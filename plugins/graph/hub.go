package graph

// GraphHub maintains the set of active clients and broadcasts messages to the clients.
type GraphHub struct {
	// Registered clients.
	clients map[*GraphClient]struct{}

	// Inbound messages from the clients.
	broadcast chan *wsMessage

	// Register requests from the clients.
	register chan *GraphClient

	// Unregister requests from clients.
	unregister chan *GraphClient

	shutdownSignal <-chan struct{}
}

func newHub() *GraphHub {
	return &GraphHub{
		clients:    make(map[*GraphClient]struct{}),
		broadcast:  make(chan *wsMessage, BROADCAST_QUEUE_SIZE),
		register:   make(chan *GraphClient),
		unregister: make(chan *GraphClient),
	}
}

func (h *GraphHub) broadcastMsg(msg *wsMessage) {
	select {
	case h.broadcast <- msg:
	default:
	}
}

func (h *GraphHub) run(shutdownSignal <-chan struct{}) {

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
