package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"

	// Правильный импорт модуля
	pb "clash-server/pb"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Структура игрока в памяти сервера
type Player struct {
	Conn     *websocket.Conn
	ID       string
	Trophies int32
}

// Пул игроков, которые ждут боя
var (
	waitingPool []*Player  // Срез (slice) ждущих игроков
	mutex       sync.Mutex // Защита памяти
)

func main() {
	// Раздаем статику (если надо) и вебсокет
	http.HandleFunc("/ws", handleConnections)

	fmt.Println("Сервер (Protobuf + Matchmaking) запущен на :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Start error:", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Ошибка апгрейда:", err)
		return
	}
	
	// В данном простом примере defer закроет соединение, когда функция завершится.
	// Мы используем select{} ниже, чтобы функция не завершалась, пока идет игра.
	defer ws.Close()

	// 1. Читаем ПЕРВОЕ сообщение от клиента (JoinQueueRequest)
	_, msg, err := ws.ReadMessage()
	if err != nil {
		log.Println("Ошибка чтения:", err)
		return
	}

	// 2. Декодируем Protobuf
	joinReq := &pb.JoinQueueRequest{}
	if err := proto.Unmarshal(msg, joinReq); err != nil {
		log.Println("Кривой Protobuf:", err)
		return
	}

	player := &Player{
		Conn:     ws,
		ID:       joinReq.PlayerId,
		Trophies: joinReq.Trophies,
	}

	fmt.Printf("Игрок %s (🏆 %d) ищет бой...\n", player.ID, player.Trophies)

	// 3. Пытаемся найти соперника
	mutex.Lock()
	opponentIndex := -1

	// Пробегаем по списку ждущих
	for i, p := range waitingPool {
		// Логика подбора: разница в кубках не больше 100
		diff := int32(math.Abs(float64(player.Trophies - p.Trophies)))
		if diff <= 100 {
			opponentIndex = i
			break
		}
	}

	if opponentIndex != -1 {
		// === НАШЛИ СОПЕРНИКА ===
		opponent := waitingPool[opponentIndex]

		// Удаляем соперника из очереди
		waitingPool = append(waitingPool[:opponentIndex], waitingPool[opponentIndex+1:]...)
		mutex.Unlock()

		// Запускаем матч
		startMatch(player, opponent)

		// Чтобы соединение player (текущего) не закрылось из-за defer,
		// здесь запускаем вечное ожидание (пока игра не закончится или сокет не отвалится).
		// В будущем тут будет игровой цикл.
		select {}

	} else {
		// === НИКОГО НЕТ, ЖДЕМ ===
		waitingPool = append(waitingPool, player)
		mutex.Unlock()
		fmt.Printf("Игрок %s добавлен в очередь ожидания.\n", player.ID)

		// Держим соединение открытым, пока нас не вызовут из startMatch
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				// Если игрок отвалился — по-хорошему надо удалить его из waitingPool
				break
			}
		}
	}
}

func startMatch(p1, p2 *Player) {
	fmt.Printf("⚔️ БОЙ: %s (%d) VS %s (%d)\n", p1.ID, p1.Trophies, p2.ID, p2.Trophies)

	roomID := fmt.Sprintf("room_%s_%s", p1.ID, p2.ID)

	// Отправляем ответ P1
	resp1 := &pb.MatchFoundResponse{
		OpponentId:       p2.ID,
		OpponentTrophies: p2.Trophies,
		RoomId:           roomID,
	}
	sendProto(p1.Conn, resp1)

	// Отправляем ответ P2
	resp2 := &pb.MatchFoundResponse{
		OpponentId:       p1.ID,
		OpponentTrophies: p1.Trophies,
		RoomId:           roomID,
	}
	sendProto(p2.Conn, resp2)
}

// Вспомогательная функция отправки Protobuf
func sendProto(conn *websocket.Conn, msg proto.Message) {
	data, err := proto.Marshal(msg)
	if err != nil {
		log.Println("Ошибка маршалинга:", err)
		return
	}
	// Важно: отправляем как BinaryMessage
	conn.WriteMessage(websocket.BinaryMessage, data)
}