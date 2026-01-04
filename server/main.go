package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	// Импортируем наш игровой движок
	// ВНИМАНИЕ: Замени 'clash-server' на имя модуля из твоего go.mod!
	"clash-server/game"
	pb "clash-server/pb"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// --- CONFIGURATION ---
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	maxMessageSize = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// --- STRUCTURES ---

type Player struct {
	Conn      *websocket.Conn
	ID        string
	Trophies  int32
	closeChan chan struct{}
}

// SessionManager
type SessionManager struct {
	sessions map[string]*Player
	mu       sync.RWMutex
}

// QueueManager
type QueueManager struct {
	pool []*Player
	mu   sync.Mutex
}

// --- GLOBALS ---

var (
	sessions = SessionManager{
		sessions: make(map[string]*Player),
	}
	queue = QueueManager{
		pool: make([]*Player, 0),
	}
)

// --- MAIN ---

func main() {
	http.HandleFunc("/ws", handleConnections)

	log.Println("CLASH SERVER STARTED :8080")
	log.Println("Mode: Authoritative Server | Elixir & Deck Logic")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Server start failed:", err)
	}
}

// --- HANDLERS ---

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	ws.SetReadLimit(maxMessageSize)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// 1. Handshake
	_, msg, err := ws.ReadMessage()
	if err != nil {
		log.Println("Handshake error:", err)
		ws.Close()
		return
	}

	joinReq := &pb.JoinQueueRequest{}
	if err := proto.Unmarshal(msg, joinReq); err != nil {
		log.Println("Proto error:", err)
		ws.Close()
		return
	}

	player := &Player{
		Conn:      ws,
		ID:        joinReq.PlayerId,
		Trophies:  joinReq.Trophies,
		closeChan: make(chan struct{}),
	}

	sessions.Register(player)

	// CLEANUP при выходе
	defer func() {
		log.Printf("Cleaning up player: %s", player.ID)
		queue.Remove(player)
		sessions.Unregister(player)
		ws.Close()
	}()

	fmt.Printf("Player %s (🏆%d) joined.\n", player.ID, player.Trophies)

	// Matchmaking
	opponent := queue.FindAndRemoveOpponent(player)
	if opponent != nil {
		startMatch(player, opponent)
	} else {
		queue.Add(player)
		log.Printf("Player %s added to queue", player.ID)
	}

	// Keep-Alive Loop
	// Мы читаем сообщения, чтобы держать сокет открытым и ловить Disconnect
	// В будущем здесь будем ловить клики "DeployCard"
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Disconnected: %s", player.ID)
			}
			break
		}
		// Тут можно добавить парсинг действий игрока (PlaceCardRequest)
	}
}

// --- GAME LOGIC ---

func startMatch(p1, p2 *Player) {
	log.Printf("MATCH START: %s vs %s", p1.ID, p2.ID)
	roomID := fmt.Sprintf("room_%s_%s", p1.ID, p2.ID)

	// 1. Отправляем MatchFound (через новую обертку ServerResponse)
	notifyMatchFound(p1, p2.ID, p2.Trophies, roomID)
	notifyMatchFound(p2, p1.ID, p1.Trophies, roomID)

	// Небольшая задержка, чтобы фронт успел переключить сцену
	time.Sleep(500 * time.Millisecond)

	// 2. Инициализируем и запускаем движок
	battle := game.NewBattle(p1.Conn, p2.Conn, p1.ID, p2.ID)

	// Запускаем в отдельной горутине, чтобы не блочить текущую
	go battle.Start()
}

func notifyMatchFound(p *Player, oppID string, oppTrophies int32, roomID string) {
	// Создаем сообщение
	resp := &pb.ServerResponse{
		Payload: &pb.ServerResponse_MatchFound{
			MatchFound: &pb.MatchFoundResponse{
				OpponentId:       oppID,
				OpponentTrophies: oppTrophies,
				RoomId:           roomID,
			},
		},
	}

	data, err := proto.Marshal(resp)
	if err != nil {
		log.Println("Marshal error:", err)
		return
	}

	p.Conn.SetWriteDeadline(time.Now().Add(writeWait))
	p.Conn.WriteMessage(websocket.BinaryMessage, data)
}

// --- SESSION & QUEUE MANAGERS (Без изменений) ---

func (sm *SessionManager) Register(p *Player) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if old, exists := sm.sessions[p.ID]; exists {
		old.Conn.Close()
	}
	sm.sessions[p.ID] = p
}

func (sm *SessionManager) Unregister(p *Player) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if stored, exists := sm.sessions[p.ID]; exists && stored == p {
		delete(sm.sessions, p.ID)
	}
}

func (qm *QueueManager) Add(p *Player) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.pool = append(qm.pool, p)
}

func (qm *QueueManager) Remove(p *Player) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	n := 0
	for _, x := range qm.pool {
		if x != p {
			qm.pool[n] = x
			n++
		}
	}
	for i := n; i < len(qm.pool); i++ {
		qm.pool[i] = nil
	}
	qm.pool = qm.pool[:n]
}

func (qm *QueueManager) FindAndRemoveOpponent(seeker *Player) *Player {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	for i, candidate := range qm.pool {
		if candidate.ID == seeker.ID {
			continue
		}
		diff := int32(math.Abs(float64(seeker.Trophies - candidate.Trophies)))
		if diff <= 1000 { // Увеличил диапазон для тестов
			qm.pool = append(qm.pool[:i], qm.pool[i+1:]...)
			return candidate
		}
	}
	return nil
}
