package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Настройка WebSocket (разрешаем подключения со всех адресов)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Структура игрока
type Player struct {
	ID   string
	Conn *websocket.Conn
}

// Глобальные переменные для матчмейкинга
var (
	waitingPlayer *Player    // Игрок, который ждет противника
	mutex         sync.Mutex // Защита от одновременной записи (чтобы сервер не упал)
)

func main() {
	http.HandleFunc("/ws", handleConnections)

	fmt.Println("Сервер запущен на :8080. Ждем игроков...")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Ошибка запуска сервера: ", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// 1. Апгрейд протокола до WebSockets
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Ошибка апгрейда соединения:", err) // Просто пишем в лог
		return                                          // И выходим из функции только для этого юзера
	}

	// Создаем нового игрока (пока просто по адресу)
	newPlayer := &Player{
		ID:   ws.RemoteAddr().String(),
		Conn: ws,
	}

	fmt.Printf("Новое подключение: %s\n", newPlayer.ID)

	// 2. Логика матчмейкинга
	mutex.Lock()
	if waitingPlayer == nil {
		// Если никого нет, этот игрок становится ждущим
		waitingPlayer = newPlayer
		newPlayer.Conn.WriteMessage(websocket.TextMessage, []byte("Ты в очереди. Ждем противника..."))
		mutex.Unlock()
	} else {
		// Если кто-то уже ждет — создаем пару!
		opponent := waitingPlayer
		waitingPlayer = nil // Очередь теперь пуста
		mutex.Unlock()

		// Уведомляем обоих, что игра началась
		startMatch(newPlayer, opponent)
	}
}

func startMatch(p1, p2 *Player) {
	fmt.Printf("Матч начался между %s и %s!\n", p1.ID, p2.ID)

	p1.Conn.WriteMessage(websocket.TextMessage, []byte("Противник найден! Бой начинается."))
	p2.Conn.WriteMessage(websocket.TextMessage, []byte("Противник найден! Бой начинается."))

	// Здесь в будущем мы запустим игровой цикл (Game Loop)
}
