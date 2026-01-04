package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"

	// Импортируем сгенерированный код (путь зависит от названия твоего модуля в go.mod)
	// Я предполагаю, что твой модуль называется "server" или "clash-backend"
	// Если будет ругаться, поменяй путь ниже на тот, что в go.mod + /pb
	pb "server/pb"

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
	defer ws.Close()

	// 1. Читаем ПЕРВОЕ сообщение от клиента.
	// Друг должен сразу после коннекта отправить JoinQueueRequest в бинарном виде.
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

		// Удаляем соперника из очереди (очень важно, чтобы срез не сломался)
		waitingPool = append(waitingPool[:opponentIndex], waitingPool[opponentIndex+1:]...)
		mutex.Unlock()

		startMatch(player, opponent)
	} else {
		// === НИКОГО НЕТ, ЖДЕМ ===
		waitingPool = append(waitingPool, player)
		mutex.Unlock()
		fmt.Printf("Игрок %s добавлен в очередь ожидания.\n", player.ID)

		// Держим соединение открытым, пока нас не вызовут из startMatch
		// В реальном проекте тут нужен канал для ожидания, но для простоты
		// пока просто бесконечный цикл чтения, чтобы сокет не закрылся.
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				// Если игрок отключился пока ждал - надо бы удалить его из waitingPool
				// Но это домашка на потом :)
				break
			}
		}
	}
}

func startMatch(p1, p2 *Player) {
	fmt.Printf("⚔️ БОЙ: %s (%d) VS %s (%d)\n", p1.ID, p1.Trophies, p2.ID, p2.Trophies)

	roomID := fmt.Sprintf("room_%s_%s", p1.ID, p2.ID)

	// Отправляем ответ P1 (что он играет против P2)
	resp1 := &pb.MatchFoundResponse{
		OpponentId:       p2.ID,
		OpponentTrophies: p2.Trophies,
		RoomId:           roomID,
	}
	sendProto(p1.Conn, resp1)

	// Отправляем ответ P2 (что он играет против P1)
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
