package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
)

var rooms = make(map[string]*Room)
var users = make(map[string]*User)

type User struct {
	UserID   string
	UserName string
}

type Room struct {
	RoomID  string
	HostID  string
	Members map[string]struct{}
}

// ERRORS

var ErrCannotJoinRoom = errors.New("Cannot Join Room")

// UTILS

func generateRandomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	return code, nil
}

// ROOM FUNCTIONS

func generateRoomID() (string, error) {
	for {
		code, err := generateRandomCode()
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
	room := &Room{
		RoomID:  room_ID,
		HostID:  host.UserID,
		Members: map[string]struct{}{host.UserID: {}},
	}

	rooms[room_ID] = room
	return room
}

func deleteRoom(roomID string) {
	room, ok := rooms[roomID]
	if !ok {
		return
	}

	for memberID := range room.Members {
		delete(users, memberID)
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
	room, ok := rooms[roomID]
	if !ok {
		return ErrCannotJoinRoom
	}
	room.AddMember(user.UserID)
	return nil
}

func leaveRoom(userID, roomID string) {
	room, ok := rooms[roomID]
	if !ok {
		return
	}
	if _, exists := room.Members[userID]; exists {
		room.DeleteMember(userID)
		delete(users, userID)
		return
	}
}

func roomStatus(roomID string) *Room {
	room, ok := rooms[roomID]
	if !ok {
		return nil
	}
	return room
}

// USER FUNCTIONS

func createUser(userName string) (*User, error) {
	for {
		code, err := generateRandomCode()
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

// HANDLERS

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Root Called")
}

func createRoomHandler(w http.ResponseWriter, r *http.Request) {
	userName := r.URL.Query().Get("userName")
	if userName == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}
	user, err := createUser(userName)
	if err != nil {
		http.Error(w, "Failed to generate User ID", http.StatusInternalServerError)
		return
	}

	room_id, err := generateRoomID()
	if err != nil {
		http.Error(w, "Failed to generate Room ID", http.StatusInternalServerError)
		return
	}

	room := createRoom(room_id, user)
	fmt.Fprintln(w, "Room Created - ID: ", room.RoomID)
}

func joinRoomHandler(w http.ResponseWriter, r *http.Request) {
	userName := r.URL.Query().Get("userName")
	roomID := r.URL.Query().Get("roomID")
	if userName == "" || roomID == "" {
		http.Error(w, "Fields required", http.StatusBadRequest)
		return
	}
	if _, exists := rooms[roomID]; exists {
		user, err := createUser(userName)
		if err != nil {
			http.Error(w, "Failed to Join Room", http.StatusInternalServerError)
			return
		}
		if joinRoom(user, roomID) != nil {
			http.Error(w, "Failed to Join Room", http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, "Joined Room")
		return
	}
	http.Error(w, "Room Not Found", http.StatusNotFound)
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
	fmt.Fprintln(w, "Room Status: Created by -", users[room.HostID].UserName, "Members -", len(room.Members))
}

// MAIN

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", rootHandler)
	mux.HandleFunc("GET /rooms/state", roomStatusHandler)
	mux.HandleFunc("POST /rooms/create", createRoomHandler)
	mux.HandleFunc("POST /rooms/join", joinRoomHandler)
	mux.HandleFunc("POST /rooms/leave", leaveRoomHandler)

	log.Println("Server Running")
	log.Fatal(http.ListenAndServe(":8000", mux))
}

// Next:
//   sync.Mutex      - race conditions
//   time.Now        - track LastSeen for cleanup
//   embed.FS        - bake the frontend into the binary
//   json.Marshal    - JSON responses
