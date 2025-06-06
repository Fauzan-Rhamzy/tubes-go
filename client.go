package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	// Get server address
	var serverAddr string
	if len(os.Args) > 1 {
		serverAddr = os.Args[1]
	} else {
		fmt.Print("Enter server address (default: localhost:8080): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			serverAddr = "localhost:8080"
		} else if !strings.Contains(input, ":") {
			serverAddr = "localhost:" + input
		} else {
			serverAddr = input
		}
	}

	fmt.Printf("Connecting to %s...\n", serverAddr)

	// Connect to server
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Printf("Cannot connect to server: %v\n", err)
		fmt.Println("Make sure the server is running and try again.")
		return
	}
	defer conn.Close()

	fmt.Println("Connected to server!")

	// Create readers
	connReader := bufio.NewReader(conn)
	localReader := bufio.NewReader(os.Stdin)

	// Handle username setup
	for {
		// Read prompt from server
		prompt, err := connReader.ReadString('\n')
		if err != nil {
			fmt.Printf("Connection error: %v\n", err)
			return
		}

		fmt.Print(prompt)

		// If server asks for username, get input and send it
		if strings.Contains(prompt, "Enter your username:") ||
			strings.Contains(prompt, "Username already taken.") ||
			strings.Contains(prompt, "Username cannot be empty.") {

			username, err := localReader.ReadString('\n')
			if err != nil {
				fmt.Printf("Error reading username: %v\n", err)
				return
			}

			conn.Write([]byte(username))

		} else if strings.Contains(prompt, "Welcome to the chat") {
			// Setup complete, break to main chat loop
			break
		}
	}

	// Show instructions
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("           CHAT APPLICATION")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("Commands:")
	fmt.Println("  /join <room>  - Join a room")
	fmt.Println("  /leave        - Leave current room")
	fmt.Println("  /rooms        - List rooms")
	fmt.Println("  /users        - List users in room")
	fmt.Println("  /help         - Show help")
	fmt.Println("  /quit         - Quit")
	fmt.Println(strings.Repeat("=", 50))

	// Start goroutine to receive messages
	go func() {
		for {
			message, err := connReader.ReadString('\n')
			if err != nil {
				fmt.Println("\nServer disconnected.")
				fmt.Println("Press Enter to exit...")
				return
			}
			fmt.Print("\r\033[K") // Clear line
			fmt.Print(message)
			fmt.Print("You: ")
		}
	}()

	// Main chat loop
	fmt.Print("You: ")
	for {
		message, err := localReader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			break
		}

		message = strings.TrimSpace(message)
		if message == "" {
			fmt.Print("You: ")
			continue
		}

		// Handle quit command
		if message == "/quit" || message == "/exit" {
			fmt.Println("Goodbye!")
			break
		}

		// Send message to server
		_, err = conn.Write([]byte(message + "\n"))
		if err != nil {
			fmt.Printf("Error sending message: %v\n", err)
			break
		}

		fmt.Print("You: ")
	}
}
