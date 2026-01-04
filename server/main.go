package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	// Убедись, что импорт соответствует твоему go.mod
	pb "clash-server/pb"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// --- CONFIGURATION ---
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096 // 4KB limit for protobuf
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// --- STRUCTURES ---

type Player struct {
	Conn     *websocket.Conn
	ID       string
	Trophies int32
	// Канал для принудительного закрытия (если зашли с другого устройства)
	closeChan chan struct{}
}

// SessionManager контролирует уникальность подключений по ID
type SessionManager struct {
	sessions map[string]*Player
	mu       sync.RWMutex
}

// QueueManager управляет очередью поиска
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

	log.Println("PRODUCTION SERVER STARTED :8080")
	log.Println("Mode: Protobuf | Strict Sessions | Graceful Cleanup")

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

	// Настройка лимитов и таймаутов для сокета (Ping/Pong)
	ws.SetReadLimit(maxMessageSize)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// 1. Ожидание handshake (JoinQueueRequest)
	// Читаем первое сообщение, чтобы понять, кто это.
	_, msg, err := ws.ReadMessage()
	if err != nil {
		log.Println("Handshake read error:", err)
		ws.Close()
		return
	}

	joinReq := &pb.JoinQueueRequest{}
	if err := proto.Unmarshal(msg, joinReq); err != nil {
		log.Println("Invalid Protobuf handshake:", err)
		ws.Close()
		return
	}

	if joinReq.PlayerId == "" {
		log.Println("Empty PlayerID rejected")
		ws.Close()
		return
	}

	player := &Player{
		Conn:      ws,
		ID:        joinReq.PlayerId,
		Trophies:  joinReq.Trophies,
		closeChan: make(chan struct{}),
	}

	// 2. Регистрация сессии (Кик дубликатов)
	// Если такой ID уже есть, старое соединение будет убито.
	sessions.Register(player)

	// 3. CLEANUP DEFER (Самое важное!)
	// Эта функция выполнится ВСЕГДА при выходе из handleConnections.
	defer func() {
		log.Printf("Cleaning up player: %s", player.ID)
		queue.Remove(player)        // Удаляем из очереди
		sessions.Unregister(player) // Удаляем из сессий
		ws.Close()                  // Закрываем сокет
	}()

	// 4. Логика Матчмейкинга
	fmt.Printf("Player %s (🏆%d) joined. Online: %d\n", player.ID, player.Trophies, sessions.Count())

	opponent := queue.FindAndRemoveOpponent(player)
	if opponent != nil {
		// Матч найден мгновенно
		startMatch(player, opponent)
	} else {
		// Добавляем в очередь
		queue.Add(player)
		log.Printf("Player %s added to queue", player.ID)
	}

	// 5. Keep-Alive Loop (Ожидание)
	// Мы запускаем чтение в цикле, чтобы ловить Ping/Pong и Disconnect.
	// Если матч начнется, этот цикл всё равно должен работать, чтобы держать соединение.
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Player %s disconnected unexpectedly: %v", player.ID, err)
			} else {
				log.Printf("Player %s disconnected", player.ID)
			}
			break // Выход из цикла -> срабатывает defer cleanup()
		}
	}
}

// --- GAME LOGIC ---

func startMatch(p1, p2 *Player) {
	log.Printf("MATCH START: %s vs %s", p1.ID, p2.ID)

	roomID := fmt.Sprintf("room_%s_%s", p1.ID, p2.ID)

	// Формируем ответы
	resp1 := &pb.MatchFoundResponse{
		OpponentId:       p2.ID,
		OpponentTrophies: p2.Trophies,
		RoomId:           roomID,
	}

	resp2 := &pb.MatchFoundResponse{
		OpponentId:       p1.ID,
		OpponentTrophies: p1.Trophies,
		RoomId:           roomID,
	}

	// Отправляем асинхронно, чтобы не блочить поток
	go sendProto(p1, resp1)
	go sendProto(p2, resp2)
}

func sendProto(p *Player, msg proto.Message) {
	data, err := proto.Marshal(msg)
	if err != nil {
		log.Println("Marshal error:", err)
		return
	}

	p.Conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := p.Conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		log.Printf("Failed to send to %s: %v", p.ID, err)
		// Если не удалось отправить - закрываем соединение, сработает cleanup
		p.Conn.Close()
	}
}

// --- SESSION MANAGER IMPLEMENTATION ---

func (sm *SessionManager) Register(p *Player) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Если сессия с таким ID уже есть — убиваем старую
	if oldPlayer, exists := sm.sessions[p.ID]; exists {
		log.Printf("Duplicate login for %s. Kicking old session.", p.ID)
		// Закрытие соединения вызовет ошибку ReadMessage в старой горутине,
		// что приведет к cleanup() старого игрока.
		oldPlayer.Conn.Close()
	}

	sm.sessions[p.ID] = p
}

func (sm *SessionManager) Unregister(p *Player) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Удаляем только если это та же самая сессия (pointer check)
	// Это защита от race condition, когда новый игрок зашел, а старый выходит
	if stored, exists := sm.sessions[p.ID]; exists && stored == p {
		delete(sm.sessions, p.ID)
	}
}

func (sm *SessionManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// --- QUEUE MANAGER IMPLEMENTATION ---

func (qm *QueueManager) Add(p *Player) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.pool = append(qm.pool, p)
}

func (qm *QueueManager) Remove(p *Player) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	// "Удаление из слайса с сохранением порядка" (Filter in-place)
	// Это O(N), но надежно для матчей.
	n := 0
	for _, x := range qm.pool {
		// Оставляем игрока, только если это НЕ тот, кого мы удаляем
		if x != p {
			qm.pool[n] = x
			n++
		}
	}
	// Обрезаем хвост (garbage collection friendly)
	for i := n; i < len(qm.pool); i++ {
		qm.pool[i] = nil
	}
	qm.pool = qm.pool[:n]
}

func (qm *QueueManager) FindAndRemoveOpponent(seeker *Player) *Player {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	for i, candidate := range qm.pool {
		// Нельзя играть с самим собой (на всякий случай)
		if candidate.ID == seeker.ID {
			continue
		}

		// Логика кубков
		diff := int32(math.Abs(float64(seeker.Trophies - candidate.Trophies)))
		if diff <= 100 {
			// Удаляем соперника из очереди
			qm.pool = append(qm.pool[:i], qm.pool[i+1:]...)
			return candidate
		}
	}
	return nil
}
