package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
)

// Client represents a connected client
type Client struct {
	conn     net.Conn
	username string
	room     string
}

// Server represents the chat server
type Server struct {
	clients   map[net.Conn]*Client
	rooms     map[string]map[net.Conn]*Client
	usernames map[string]bool
	mutex     sync.RWMutex
	listener  net.Listener
}

// NewServer creates a new server instance
func NewServer() *Server {
	return &Server{
		clients:   make(map[net.Conn]*Client),
		rooms:     make(map[string]map[net.Conn]*Client),
		usernames: make(map[string]bool),
	}
}

// Start starts the server on the specified port
func (s *Server) Start(port string) error {
	var err error
	s.listener, err = net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	fmt.Printf("Chat server started on port %s\n", port)
	fmt.Println("Waiting for clients to connect...")

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}

		// Handle each client connection in a separate goroutine
		go s.handleClient(conn)
	}
}

// handleClient handles individual client connections
func (s *Server) handleClient(conn net.Conn) {
	defer conn.Close()

	// Get username from client
	username, err := s.getUsernameFromClient(conn)
	if err != nil {
		// Only log if it's not a simple disconnect
		if !strings.Contains(err.Error(), "disconnected") {
			fmt.Printf("Failed to get username: %v\n", err)
		} else {
			fmt.Printf("Client disconnected during username setup\n")
		}
		return
	}

	// Create client instance
	client := &Client{
		conn:     conn,
		username: username,
		room:     "general", // Default room
	}

	// Add client to server
	s.addClient(client)

	// Send welcome message
	s.sendToClient(client, "SERVER", fmt.Sprintf("Welcome to the chat, %s! You are in room 'general'", username))

	// Notify other clients about new user
	s.broadcastToRoom("general", "SERVER", fmt.Sprintf("%s has joined the chat", username), client.conn)

	// Handle client messages
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message := strings.TrimSpace(scanner.Text())
		if message == "" {
			continue
		}

		// Handle special commands
		if strings.HasPrefix(message, "/") {
			s.handleCommand(client, message)
		} else {
			// Broadcast regular message to room
			s.broadcastToRoom(client.room, client.username, message, client.conn)
		}
	}
 
	// Client disconnected
	s.removeClient(client)
	s.broadcastToRoom(client.room, "SERVER", fmt.Sprintf("%s has left the chat", username), nil)
	fmt.Printf("Client %s disconnected\n", username)
}

// getUsernameFromClient gets username from client and validates it
func (s *Server) getUsernameFromClient(conn net.Conn) (string, error) {
	conn.Write([]byte("Enter your username: \n"))

	reader := bufio.NewReader(conn)
	for {
		username, err := reader.ReadString('\n')
		if err != nil {
			if err.Error() == "EOF" {
				return "", fmt.Errorf("client disconnected during username setup")
			}
			return "", fmt.Errorf("connection error while reading username: %v", err)
		}

		username = strings.TrimSpace(username)

		if username == "" {
			conn.Write([]byte("Username cannot be empty. Enter your username: \n"))
			continue
		}

		s.mutex.Lock()
		if s.usernames[username] {
			s.mutex.Unlock()
			conn.Write([]byte("Username already taken. Enter your username: \n"))
			continue
		}
		s.usernames[username] = true
		s.mutex.Unlock()

		return username, nil
	}
}

// addClient adds a client to the server
func (s *Server) addClient(client *Client) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.clients[client.conn] = client

	// Add to room
	if s.rooms[client.room] == nil {
		s.rooms[client.room] = make(map[net.Conn]*Client)
	}
	s.rooms[client.room][client.conn] = client

	fmt.Printf("Client %s connected (Total clients: %d)\n", client.username, len(s.clients))
}

// removeClient removes a client from the server
func (s *Server) removeClient(client *Client) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Remove from clients map
	delete(s.clients, client.conn)

	// Remove from room
	if s.rooms[client.room] != nil {
		delete(s.rooms[client.room], client.conn)
		if len(s.rooms[client.room]) == 0 {
			delete(s.rooms, client.room)
		}
	}

	// Remove username
	delete(s.usernames, client.username)

	fmt.Printf("Client %s removed (Total clients: %d)\n", client.username, len(s.clients))
}

// broadcastToRoom broadcasts a message to all clients in a specific room except sender
func (s *Server) broadcastToRoom(room, sender, message string, excludeConn net.Conn) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.rooms[room] == nil {
		return
	}

	formattedMessage := fmt.Sprintf("[%s] %s: %s", room, sender, message)

	for conn, client := range s.rooms[room] {
		if conn != excludeConn {
			s.sendToClient(client, "", formattedMessage)
		}
	}
}

// sendToClient sends a message to a specific client
func (s *Server) sendToClient(client *Client, sender, message string) {
	var formattedMessage string
	if sender == "" {
		formattedMessage = message + "\n"
	} else {
		formattedMessage = fmt.Sprintf("[%s] %s: %s\n", client.room, sender, message)
	}

	_, err := client.conn.Write([]byte(formattedMessage))
	if err != nil {
		fmt.Printf("Failed to send message to %s: %v\n", client.username, err)
	}
}

// handleCommand handles special commands from clients
func (s *Server) handleCommand(client *Client, command string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/join":
		if len(parts) < 2 {
			s.sendToClient(client, "SERVER", "Usage: /join <room_name>")
			return
		}
		s.joinRoom(client, parts[1])

	case "/leave":
		s.leaveRoom(client)

	case "/rooms":
		s.listRooms(client)

	case "/users":
		s.listUsersInRoom(client)

	case "/help":
		s.showHelp(client)

	default:
		s.sendToClient(client, "SERVER", "Unknown command. Type /help for available commands.")
	}
}

// joinRoom moves a client to a different room
func (s *Server) joinRoom(client *Client, newRoom string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	oldRoom := client.room

	// Remove from old room
	if s.rooms[oldRoom] != nil {
		delete(s.rooms[oldRoom], client.conn)
		if len(s.rooms[oldRoom]) == 0 && oldRoom != "general" {
			delete(s.rooms, oldRoom)
		}
	}

	// Add to new room
	if s.rooms[newRoom] == nil {
		s.rooms[newRoom] = make(map[net.Conn]*Client)
	}
	s.rooms[newRoom][client.conn] = client
	client.room = newRoom

	// Notify rooms about user movement
	go s.broadcastToRoom(oldRoom, "SERVER", fmt.Sprintf("%s left the room", client.username), nil)
	go s.broadcastToRoom(newRoom, "SERVER", fmt.Sprintf("%s joined the room", client.username), client.conn)

	s.sendToClient(client, "SERVER", fmt.Sprintf("You joined room '%s'", newRoom))
}

// leaveRoom moves a client back to general room
func (s *Server) leaveRoom(client *Client) {
	if client.room == "general" {
		s.sendToClient(client, "SERVER", "You are already in the general room")
		return
	}
	s.joinRoom(client, "general")
}

// listRooms lists all available rooms
func (s *Server) listRooms(client *Client) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if len(s.rooms) == 0 {
		s.sendToClient(client, "SERVER", "No active rooms")
		return
	}

	roomList := "Active rooms:\n"
	for room, clients := range s.rooms {
		roomList += fmt.Sprintf("  - %s (%d users)\n", room, len(clients))
	}
	s.sendToClient(client, "SERVER", roomList)
}

// listUsersInRoom lists all users in current room
func (s *Server) listUsersInRoom(client *Client) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.rooms[client.room] == nil {
		s.sendToClient(client, "SERVER", "No users in this room")
		return
	}

	userList := fmt.Sprintf("Users in room '%s':\n", client.room)
	for _, roomClient := range s.rooms[client.room] {
		userList += fmt.Sprintf("  - %s\n", roomClient.username)
	}
	s.sendToClient(client, "SERVER", userList)
}

// showHelp shows available commands
func (s *Server) showHelp(client *Client) {
	help := `Available commands:
  /join <room>  - Join a specific room
  /leave        - Leave current room (return to general)
  /rooms        - List all active rooms
  /users        - List users in current room
  /help         - Show this help message
  /quit         - Disconnect from the server`
	s.sendToClient(client, "SERVER", help)
}

func main() {
	server := NewServer()

	// Start server on port 8080
	err := server.Start("8080")
	if err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
