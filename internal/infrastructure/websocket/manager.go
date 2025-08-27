package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ConnectionStatus represents the status of a WebSocket connection
type ConnectionStatus string

const (
	StatusConnecting ConnectionStatus = "connecting"
	StatusConnected  ConnectionStatus = "connected"
	StatusClosed     ConnectionStatus = "closed"
	StatusError      ConnectionStatus = "error"
)

// Message represents a WebSocket message
type Message struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	ConversationID string                `json:"conversation_id,omitempty"`
	Content       string                 `json:"content,omitempty"`
	Role          string                 `json:"role,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	Error         string                 `json:"error,omitempty"`
}

// Connection represents a WebSocket connection
type Connection struct {
	ID       string
	UserID   string
	Conn     *websocket.Conn
	Send     chan Message
	Status   ConnectionStatus
	LastSeen time.Time
	mutex    sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewConnection creates a new WebSocket connection
func NewConnection(id, userID string, conn *websocket.Conn) *Connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &Connection{
		ID:       id,
		UserID:   userID,
		Conn:     conn,
		Send:     make(chan Message, 256),
		Status:   StatusConnecting,
		LastSeen: time.Now(),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// UpdateStatus safely updates the connection status
func (c *Connection) UpdateStatus(status ConnectionStatus) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.Status = status
	c.LastSeen = time.Now()
}

// GetStatus safely gets the connection status
func (c *Connection) GetStatus() ConnectionStatus {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.Status
}

// Close closes the connection
func (c *Connection) Close() {
	c.UpdateStatus(StatusClosed)
	c.cancel()
	close(c.Send)
	c.Conn.Close()
}

// Manager manages WebSocket connections
type Manager struct {
	connections    map[string]*Connection
	register       chan *Connection
	unregister     chan *Connection
	broadcast      chan Message
	mutex          sync.RWMutex
	upgrader       websocket.Upgrader
	messageHandler MessageHandler
}

// NewManager creates a new WebSocket manager
func NewManager() *Manager {
	return &Manager{
		connections: make(map[string]*Connection),
		register:    make(chan *Connection),
		unregister:  make(chan *Connection),
		broadcast:   make(chan Message),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// In production, implement proper origin checking
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// Start starts the WebSocket manager
func (m *Manager) Start(ctx context.Context) {
	// Start connection cleanup goroutine
	go m.cleanup(ctx)
	
	for {
		select {
		case <-ctx.Done():
			return
		case conn := <-m.register:
			m.registerConnection(conn)
		case conn := <-m.unregister:
			m.unregisterConnection(conn)
		case message := <-m.broadcast:
			m.broadcastMessage(message)
		}
	}
}

// registerConnection registers a new connection
func (m *Manager) registerConnection(conn *Connection) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.connections[conn.ID] = conn
	conn.UpdateStatus(StatusConnected)
	
	log.Printf("WebSocket connection registered: %s (User: %s)", conn.ID, conn.UserID)
	
	// Send welcome message
	welcome := Message{
		ID:        uuid.New().String(),
		Type:      "connection_established",
		Content:   "Connection established successfully",
		Timestamp: time.Now(),
	}
	
	select {
	case conn.Send <- welcome:
	default:
		close(conn.Send)
		delete(m.connections, conn.ID)
	}
}

// unregisterConnection unregisters a connection
func (m *Manager) unregisterConnection(conn *Connection) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if _, exists := m.connections[conn.ID]; exists {
		delete(m.connections, conn.ID)
		conn.Close()
		log.Printf("WebSocket connection unregistered: %s", conn.ID)
	}
}

// broadcastMessage broadcasts a message to relevant connections
func (m *Manager) broadcastMessage(message Message) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	for _, conn := range m.connections {
		select {
		case conn.Send <- message:
		default:
			// Connection is closed, clean it up
			close(conn.Send)
			delete(m.connections, conn.ID)
		}
	}
}

// SendToUser sends a message to a specific user
func (m *Manager) SendToUser(userID string, message Message) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	for _, conn := range m.connections {
		if conn.UserID == userID {
			select {
			case conn.Send <- message:
				return nil
			default:
				return ErrConnectionClosed
			}
		}
	}
	return ErrUserNotConnected
}

// SendToConnection sends a message to a specific connection
func (m *Manager) SendToConnection(connectionID string, message Message) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if conn, exists := m.connections[connectionID]; exists {
		select {
		case conn.Send <- message:
			return nil
		default:
			return ErrConnectionClosed
		}
	}
	return ErrConnectionNotFound
}

// GetUserConnections returns all connections for a user
func (m *Manager) GetUserConnections(userID string) []*Connection {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	var connections []*Connection
	for _, conn := range m.connections {
		if conn.UserID == userID {
			connections = append(connections, conn)
		}
	}
	return connections
}

// GetConnectionCount returns the total number of active connections
func (m *Manager) GetConnectionCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.connections)
}

// cleanup removes stale connections
func (m *Manager) cleanup(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.removeStaleConnections()
		}
	}
}

// removeStaleConnections removes connections that haven't been active recently
func (m *Manager) removeStaleConnections() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	cutoff := time.Now().Add(-5 * time.Minute)
	for id, conn := range m.connections {
		if conn.LastSeen.Before(cutoff) || conn.GetStatus() == StatusClosed {
			conn.Close()
			delete(m.connections, id)
			log.Printf("Removed stale connection: %s", id)
		}
	}
}

// HandleUpgrade handles WebSocket upgrade requests
func (m *Manager) HandleUpgrade(w http.ResponseWriter, r *http.Request, userID string) error {
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	
	connectionID := uuid.New().String()
	wsConn := NewConnection(connectionID, userID, conn)
	
	// Register the connection
	m.register <- wsConn
	
	// Start goroutines for reading and writing
	go m.readPump(wsConn)
	go m.writePump(wsConn)
	
	return nil
}

// readPump pumps messages from the websocket connection to the hub
func (m *Manager) readPump(conn *Connection) {
	defer func() {
		m.unregister <- conn
		conn.Conn.Close()
	}()
	
	conn.Conn.SetReadLimit(512 * 1024) // 512KB
	conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.Conn.SetPongHandler(func(string) error {
		conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.LastSeen = time.Now()
		return nil
	})
	
	for {
		select {
		case <-conn.ctx.Done():
			return
		default:
			var message Message
			err := conn.Conn.ReadJSON(&message)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error: %v", err)
				}
				return
			}
			
			conn.LastSeen = time.Now()
			message.Timestamp = time.Now()
			
			// Handle the message (you would typically send this to a message handler)
			m.handleIncomingMessage(conn, message)
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (m *Manager) writePump(conn *Connection) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Conn.Close()
	}()
	
	for {
		select {
		case <-conn.ctx.Done():
			return
		case message, ok := <-conn.Send:
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			
			if err := conn.Conn.WriteJSON(message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
			
		case <-ticker.C:
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// MessageHandler interface for processing incoming messages
type MessageHandler interface {
	HandleMessage(ctx context.Context, userID string, message Message) error
}

// SetMessageHandler sets the message handler for processing incoming messages
func (m *Manager) SetMessageHandler(handler MessageHandler) {
	m.messageHandler = handler
}

// handleIncomingMessage handles incoming messages from clients
func (m *Manager) handleIncomingMessage(conn *Connection, message Message) {
	// Set message ID if not provided
	if message.ID == "" {
		message.ID = uuid.New().String()
	}
	
	// Log the message
	log.Printf("Received message from connection %s: %s", conn.ID, message.Type)
	
	switch message.Type {
	case "ping":
		response := Message{
			ID:        uuid.New().String(),
			Type:      "pong",
			Timestamp: time.Now(),
		}
		conn.Send <- response
		
	case "chat_message":
		// Process chat message with handler if available
		if m.messageHandler != nil {
			ctx := context.Background()
			if err := m.messageHandler.HandleMessage(ctx, conn.UserID, message); err != nil {
				log.Printf("Error handling message: %v", err)
				
				// Send error response to client
				errorResponse := Message{
					ID:        message.ID,
					Type:      "error",
					Error:     "Failed to process message",
					Timestamp: time.Now(),
				}
				select {
				case conn.Send <- errorResponse:
				default:
					log.Printf("Failed to send error response to connection %s", conn.ID)
				}
			}
		} else {
			log.Printf("No message handler configured - echoing message")
			
			// Simple echo response when no handler is available
			echoResponse := Message{
				ID:        message.ID,
				Type:      "chat_response",
				Content:   fmt.Sprintf("Echo: %s", message.Content),
				Role:      "assistant",
				Timestamp: time.Now(),
			}
			
			select {
			case conn.Send <- echoResponse:
			default:
				log.Printf("Failed to send echo response to connection %s", conn.ID)
			}
		}
		
	default:
		log.Printf("Unknown message type: %s", message.Type)
	}
}

// Custom errors
var (
	ErrConnectionClosed   = &WebSocketError{Code: "CONNECTION_CLOSED", Message: "Connection is closed"}
	ErrUserNotConnected   = &WebSocketError{Code: "USER_NOT_CONNECTED", Message: "User is not connected"}
	ErrConnectionNotFound = &WebSocketError{Code: "CONNECTION_NOT_FOUND", Message: "Connection not found"}
)

// WebSocketError represents a WebSocket-specific error
type WebSocketError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *WebSocketError) Error() string {
	return e.Message
}

// MarshalJSON implements json.Marshaler
func (e *WebSocketError) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{
		Code:    e.Code,
		Message: e.Message,
	})
}