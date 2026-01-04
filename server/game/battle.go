package game

import (
	"log"
	"math/rand"
	"sync"
	"time"

	pb "clash-server/pb" // Проверь, что путь совпадает с go.mod

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

const (
	MaxElixir  = 10.0
	TickRate   = 100 * time.Millisecond // 10 раз в секунду
	ElixirRate = 0.0357                 // ~1 эликсир за 2.8 сек (при тике 0.1с)
)

// PlayerState - состояние игрока внутри матча
type PlayerState struct {
	Conn     *websocket.Conn
	ID       string
	Elixir   float64
	Deck     []int32 // Вся колода
	Hand     []int32 // Текущие 4 карты
	NextCard int32   // Следующая карта
	Queue    []int32 // Оставшиеся в колоде
	mu       sync.Mutex
}

// Battle - экземпляр одной игры
type Battle struct {
	P1       *PlayerState
	P2       *PlayerState
	stopChan chan struct{}
	tick     int32
}

// NewBattle создает новую игру
func NewBattle(p1Conn, p2Conn *websocket.Conn, p1ID, p2ID string) *Battle {
	return &Battle{
		P1:       initPlayer(p1Conn, p1ID),
		P2:       initPlayer(p2Conn, p2ID),
		stopChan: make(chan struct{}),
		tick:     0,
	}
}

func initPlayer(conn *websocket.Conn, id string) *PlayerState {
	// 1. Создаем колоду [1..8]
	deck := make([]int32, 8)
	for i := 0; i < 8; i++ {
		deck[i] = int32(i + 1)
	}

	// 2. Мешаем (Fisher-Yates shuffle)
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })

	// 3. Раздаем
	return &PlayerState{
		Conn:     conn,
		ID:       id,
		Elixir:   5.0, // Старт с 5 эликсира
		Deck:     deck,
		Hand:     deck[0:4], // Первые 4
		NextCard: deck[4],   // 5-я карта
		Queue:    deck[5:],  // Остальные
	}
}

// Start запускает цикл игры
func (b *Battle) Start() {
	ticker := time.NewTicker(TickRate)
	defer ticker.Stop()

	log.Printf("GAME LOOP STARTED: %s vs %s", b.P1.ID, b.P2.ID)

	for {
		select {
		case <-b.stopChan:
			return
		case <-ticker.C:
			b.tick++
			b.Update()
			b.Broadcast()
		}
	}
}

// Update начисляет эликсир
func (b *Battle) Update() {
	updateElixir(b.P1)
	updateElixir(b.P2)
}

func updateElixir(p *PlayerState) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Elixir < MaxElixir {
		p.Elixir += ElixirRate
		if p.Elixir > MaxElixir {
			p.Elixir = MaxElixir
		}
	}
}

// Broadcast отправляет состояние клиентам
func (b *Battle) Broadcast() {
	sendState(b.P1, b.tick)
	sendState(b.P2, b.tick)
}

func sendState(p *PlayerState, tick int32) {
	p.mu.Lock()
	state := &pb.GameStateUpdate{
		Elixir:     float32(p.Elixir),
		Hand:       p.Hand,
		NextCard:   p.NextCard,
		ServerTick: tick,
	}
	p.mu.Unlock()

	// Оборачиваем в ServerResponse
	wrapper := &pb.ServerResponse{
		Payload: &pb.ServerResponse_GameState{
			GameState: state,
		},
	}

	data, err := proto.Marshal(wrapper)
	if err != nil {
		log.Println("Proto marshal error:", err)
		return
	}

	// Отправляем
	// Важно: в реальном проекте здесь нужен Mutex на Conn, так как чтение идет в main.go
	if err := p.Conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		// Ошибки записи обычно означают разрыв соединения,
		// это обработается в main.go
		return
	}
}
