package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Player struct {
	ID           string
	Conn         *websocket.Conn
	LastActivity time.Time
	mu           sync.Mutex
}

type GameServer struct {
	players     map[string]*Player
	playersMu   sync.RWMutex
	upgrader    websocket.Upgrader
	maxPlayers  int
	readTimeout time.Duration
}

type MessageType string

type StructuredMessage struct {
	Type      MessageType     `json:"type"`
	PlayerID  string          `json:"player_id"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
}

// Examples of message types
const (
	PlayerMove    MessageType = "PLAYER_MOVE"
	GameStateSync MessageType = "GAME_STATE_SYNC"
	PlayerJoin    MessageType = "PLAYER_JOIN"
	PlayerLeave   MessageType = "PLAYER_LEAVE"
	ChatMessage   MessageType = "CHAT_MESSAGE"
)

func NewGameServer(maxPlayers int) *GameServer {
	return &GameServer{
		players:     make(map[string]*Player),
		maxPlayers:  maxPlayers,
		readTimeout: 10 * time.Minute,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Customize origin checking if needed
				// Customizing origin checking is necessary for security reasons.
				// It helps prevent Cross-Site WebSocket Hijacking (CSWSH) attacks,
				// where a malicious website could attempt to connect to your WebSocket server from an unauthorized origin.
				// By implementing a custom CheckOrigin function, you can restrict connections to only trusted origins.
				// Here is an example of how you might customize the CheckOrigin function to allow only specific origins:
				//
				// CheckOrigin: func(r *http.Request) bool {
				// 	origin := r.Header.Get("Origin")
				// 	// Allow only specific origins
				// 	if origin == "http://trusted-origin.com" || origin == "http://another-trusted-origin.com" {
				// 		return true
				// 	}
				// 	return false
				// }
				return true
			},
		},
	}
}

func (gs *GameServer) RegisterPlayer(conn *websocket.Conn) (*Player, error) {
	gs.playersMu.Lock()
	defer gs.playersMu.Unlock()

	if len(gs.players) >= gs.maxPlayers {
		return nil, fmt.Errorf("server is full")
	}

	playerID := generateUniqueID()
	player := &Player{
		ID:           playerID,
		Conn:         conn,
		LastActivity: time.Now(),
	}

	gs.players[playerID] = player
	log.Printf("Player %s connected", playerID)
	return player, nil
}

func (gs *GameServer) UnregisterPlayer(playerID string) {
	gs.playersMu.Lock()
	defer gs.playersMu.Unlock()

	if player, exists := gs.players[playerID]; exists {
		player.Conn.Close()
		delete(gs.players, playerID)
		log.Printf("Player %s disconnected", playerID)
	}
}

// BroadcastMessage sends a message to all connected players
// This is just for raw text messages, and they are sent to all the players
func (gs *GameServer) BroadcastMessage(message []byte) {
	gs.playersMu.RLock()
	defer gs.playersMu.RUnlock()

	for _, player := range gs.players {
		player.mu.Lock()
		err := player.Conn.WriteMessage(websocket.TextMessage, message)
		player.mu.Unlock()

		if err != nil {
			log.Printf("Error broadcasting to player %s: %v", player.ID, err)
		}
	}
}

func (gs *GameServer) SendStructuredMessage(playerID string, msgType MessageType, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	msg := StructuredMessage{
		Type:      msgType,
		PlayerID:  playerID,
		Payload:   payloadBytes,
		Timestamp: time.Now().Unix(),
	}

	// Convert entire message to bytes
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	// Find and send to specific player
	gs.playersMu.RLock()
	player, exists := gs.players[playerID]
	gs.playersMu.RUnlock()

	if !exists {
		return fmt.Errorf("player not found")
	}

	return player.Conn.WriteMessage(websocket.TextMessage, msgBytes)
}

// HandlePlayerMessages handles incoming messages from a player
func (gs *GameServer) HandlePlayerMessages(player *Player) {
	defer gs.UnregisterPlayer(player.ID)

	for {
		player.Conn.SetReadDeadline(time.Now().Add(gs.readTimeout))

		_, message, err := player.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected close error for player %s: %v", player.ID, err)
			}
			break
		}

		if err := gs.processMessage(player, message); err != nil {
			log.Printf("Message processing error: %v", err)
		}

		// fmt.Println("Player " + player.ID + " sent the message with the content: " + string(message))

		player.LastActivity = time.Now()
	}
}

// processTextMessage handles text-based game messages
func (gs *GameServer) processTextMessage(player *Player, message []byte) {
	// Implement your game-specific message processing logic here
	log.Printf("Received text message from %s: %s", player.ID, string(message))

	// Example: Echo message back to all players
	gs.BroadcastMessage(message)
}

// processMessage handles structured messages with type-based routing
func (gs *GameServer) processMessage(player *Player, data []byte) error {
	var msg StructuredMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("invalid message format")
	}

	// Example message type handling
	switch msg.Type {
	case PlayerMove:
		// Decode and process player movement
		// Example: var moveData PlayerMovePayload
		// json.Unmarshal(msg.Payload, &moveData)
		log.Printf("Player %s moved", player.ID)

	case ChatMessage:
		// Broadcast chat message to all players
		gs.BroadcastMessage(data)

	case GameStateSync:
		// Validate and update game state
		log.Printf("Game state sync from player %s", player.ID)

	// Can have more if needed
	default:
		log.Printf("Unhandled message type: %s", msg.Type)
	}

	return nil
}

func (gs *GameServer) processBinaryMessage(player *Player, message []byte) {
	// Implement your game-specific binary message processing logic
	log.Printf("Received binary message from %s (length: %d)", player.ID, len(message))
}

func (gs *GameServer) StartServer(addr string) error {
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := gs.upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		player, err := gs.RegisterPlayer(conn)
		if err != nil {
			log.Printf("Player registration error: %v", err)
			conn.Close()
			return
		}

		go gs.HandlePlayerMessages(player)
	})

	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, nil)
}

func generateUniqueID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func main() {
	maxPlayerNumber := 100
	server := NewGameServer(maxPlayerNumber)
	err := server.StartServer(":8080")
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
