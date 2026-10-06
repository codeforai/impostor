package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"net/http"
	"sync"
	"time"
)

const (
	MaxPlayerLimit = 20
	DigitsInUserID = 16
	DigitsInRoomID = 6
)

type GameMode string

const (
	Standard       GameMode = "standard"
	NoImposter     GameMode = "noImposter"
	OnlyImposters  GameMode = "onlyImposters"
	DoubleImposter GameMode = "doubleImposter"
)

type GamePhase string

const (
	Lobby      GamePhase = "lobby"
	Reveal     GamePhase = "reveal"
	Discussion GamePhase = "discussion"
	Voting     GamePhase = "voting"
	Results    GamePhase = "results"
)

var (
	mu    sync.Mutex
	rooms = make(map[string]*Room)
	users = make(map[string]*User)
)

type User struct {
	UserID   string
	UserName string
	LastSeen time.Time
}

type GameConfig struct {
}

type GameState struct {
}

type Room struct {
	RoomID       string
	HostID       string
	Members      map[string]struct{}
	Config       GameConfig
	State        *GameState
	LastAccessed time.Time
}

// ERRORS

var ErrRoomNotFound = errors.New("Room Not Found")
var ErrRoomFull = errors.New("Room Is Full")

// ENUMS

func (gm GameMode) isValidGameMode() bool {
	switch gm {
	case Standard, OnlyImposters, NoImposter, DoubleImposter:
		return true
	default:
		return false
	}
}

func (gp GamePhase) isValidGamePhase() bool {
	switch gp {
	case Lobby, Reveal, Discussion, Voting, Results:
		return true
	default:
		return false
	}
}

// UTILS

func generateRandomCode(digits int) (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(math.Pow10(digits))))
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	return code, nil
}

// ROOM FUNCTIONS

func generateRoomID() (string, error) {
	for {
		code, err := generateRandomCode(DigitsInRoomID)
		if err != nil {
			return "", err
		}
		if _, exists := rooms[code]; exists {
			continue
		}
		return code, nil
	}

}

func createRoom(room_ID string, host *User) *Room {
	mu.Lock()
	defer mu.Unlock()

	room := &Room{
		RoomID:  room_ID,
		HostID:  host.UserID,
		Members: map[string]struct{}{host.UserID: {}},
	}

	rooms[room_ID] = room
	return room
}

func deleteRoom(roomID string) {
	mu.Lock()
	defer mu.Unlock()

	_, ok := rooms[roomID]
	if !ok {
		return
	}
	delete(rooms, roomID)
}

func (r *Room) AddMember(userID string) {
	r.Members[userID] = struct{}{}
}

func (r *Room) DeleteMember(userID string) {
	if userID == r.HostID {
		deleteRoom(r.RoomID)
		return
	}
	delete(r.Members, userID)
}

func joinRoom(user *User, roomID string) error {
	mu.Lock()
	defer mu.Unlock()

	room, ok := rooms[roomID]
	if !ok {
		return ErrRoomNotFound
	}
	if len(room.Members) >= MaxPlayerLimit {
		return ErrRoomFull
	}
	room.AddMember(user.UserID)
	return nil
}

func leaveRoom(userID, roomID string) {
	mu.Lock()
	defer mu.Unlock()

	room, ok := rooms[roomID]
	if !ok {
		return
	}
	if _, exists := room.Members[userID]; exists {
		room.DeleteMember(userID)
		return
	}
}

func roomStatus(roomID string) *Room {
	mu.Lock()
	defer mu.Unlock()

	room, ok := rooms[roomID]
	if !ok {
		return nil
	}
	return room
}

// USER FUNCTIONS

func createUser(userName string) (*User, error) {
	mu.Lock()
	defer mu.Unlock()

	for {
		code, err := generateRandomCode(DigitsInUserID)
		if err != nil {
			return nil, err
		}
		if _, exists := users[code]; exists {
			continue
		}

		user := &User{
			UserID:   code,
			UserName: userName,
		}

		users[code] = user
		return user, nil
	}
}

func getUserByID(userID string) *User {
	mu.Lock()
	defer mu.Unlock()

	user, ok := users[userID]
	if !ok {
		return nil
	}
	return user
}

// HANDLERS

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Root Called")
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	userName := r.URL.Query().Get("userName")
	if userName == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}
	user, err := createUser(userName)
	if err != nil {
		http.Error(w, "Failed to create User", http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, "User Created - ID: ", user.UserID, user.UserName)

}

func createRoomHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userID")
	if userID == "" {
		http.Error(w, "UserID required", http.StatusBadRequest)
		return
	}
	user := getUserByID(userID)

	if user == nil {
		http.Error(w, "User Not Found", http.StatusNotFound)
		return
	}

	room_id, err := generateRoomID()
	if err != nil {
		http.Error(w, "Failed to generate Room ID", http.StatusInternalServerError)
		return
	}

	room := createRoom(room_id, user)
	if room == nil {
		http.Error(w, "Failed to create Room", http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, "Room Created - ID: ", room.RoomID)
}

func joinRoomHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userID")
	roomID := r.URL.Query().Get("roomID")
	if userID == "" || roomID == "" {
		http.Error(w, "Fields required", http.StatusBadRequest)
		return
	}
	user := getUserByID(userID)
	if user == nil {
		http.Error(w, "User Not Found", http.StatusNotFound)
		return
	}
	if err := joinRoom(user, roomID); err != nil {
		switch {
		case errors.Is(err, ErrRoomNotFound):
			http.Error(w, "Failed to Join Room", http.StatusNotFound)
		case errors.Is(err, ErrRoomFull):
			http.Error(w, "Failed to Join Room", http.StatusConflict)
		default:
			http.Error(w, "Failed to join", http.StatusInternalServerError)
		}
		return
	}
	fmt.Fprintln(w, "Joined Room")
	return
}

func leaveRoomHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userID")
	roomID := r.URL.Query().Get("roomID")
	leaveRoom(userID, roomID)
	fmt.Fprintln(w, "Left Room")
}

func roomStatusHandler(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("roomID")
	if roomID == "" {
		http.Error(w, "Fields required", http.StatusBadRequest)
		return
	}
	room := roomStatus(roomID)
	if room == nil {
		http.Error(w, "Error", http.StatusNotFound)
		return
	}
	user := getUserByID(room.HostID)
	if user == nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, "Room Status: Created by -", user.UserName, "Members -", len(room.Members))
}

func roomConfigHandler(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("roomID")
	userID := r.URL.Query().Get("userID")
	if roomID == "" || userID == "" {
		http.Error(w, "Fields Required", http.StatusBadRequest)
		return
	}
	room := roomStatus(roomID)
	if room == nil {
		http.Error(w, "Error", http.StatusNotFound)
		return
	}
	if userID != room.HostID {
		http.Error(w, "Error", http.StatusUnauthorized)
		return
	}
	// change settings
}

func startGameHandler(w http.ResponseWriter, r *http.Request) {
	// start game logic
}

// MAIN

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", rootHandler)
	mux.HandleFunc("GET /rooms/state", roomStatusHandler)

	mux.HandleFunc("POST /rooms/create", createRoomHandler)
	mux.HandleFunc("POST /rooms/join", joinRoomHandler)
	mux.HandleFunc("POST /rooms/leave", leaveRoomHandler)
	mux.HandleFunc("POST /rooms/config", roomConfigHandler)
	mux.HandleFunc("POST /rooms/start", startGameHandler)

	mux.HandleFunc("POST /users/create", createUserHandler)

	log.Println("Server Running")
	log.Fatal(http.ListenAndServe(":8000", mux))
}

// Next:
//   mutex in functions
//   embed.FS        - bake the frontend into the binary
//   json.Marshal    - JSON responses
