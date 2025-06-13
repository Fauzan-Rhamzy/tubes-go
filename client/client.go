package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {

	// Dapatkan server address dari user
	var serverAddr string
	fmt.Print("Enter server address (default: localhost:9090): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		serverAddr = "localhost:9090" // Jika kosong maka gunakan default address
	} else {
		serverAddr = input // Jika tidak maka gunakan address dari user
	}

	fmt.Printf("Connecting to %s...\n", serverAddr)

	// Koneksi ke server
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Printf("Cannot connect to server: %v\n", err)
		fmt.Println("Make sure the server is running and try again.")
		return
	}

	defer conn.Close() //tutup koneksi jika sudah selesai

	fmt.Println("Connected to server!")

	connReader := bufio.NewReader(conn)      // Reader untuk server
	localReader := bufio.NewReader(os.Stdin) // Reader untuk lokal

	// Meminta username dari client
	fmt.Print("Masukkan username: ")
	nameInput, _ := localReader.ReadString('\n') // Input dari username dari client
	conn.Write([]byte(nameInput))                // Kirim username yang telah diketik ke server

	name := strings.TrimSpace(nameInput)       // Simpan username milik client
	prompt := fmt.Sprintf("(%s) You > ", name) // Prompt untuk client contoh (username) You >

	// Mulai go routine untuk menerima pesan dari server secara terus menerus
	go func() {
		for {

			message, err := connReader.ReadString('\n') // Menunggu pesan dari server

			// Jika koneksi terputus beri tahu user dan keluar
			if err != nil {
				fmt.Println("\nServer disconnected. Press Enter to exit.")
				os.Exit(0) // Terminate the client program
			}

			fmt.Print("\r\033[K") // Menghapus line yang ada di console
			fmt.Print(message)    // Print pesan yang didapat dari server
			fmt.Print(prompt)     // Tampilkan kembali prompt
		}
	}()

	// The server will now send back a list of rooms. The goroutine above
	// will handle displaying it and will show the first prompt.

	/*
		Setelah user menggunakan username yang valid,
		User akan ditempatkan di room 'general' terlebih dahulu,
		Lalu server akan mengirim list of room,
		hal - hal tersebut akan di tangani oleh go routine diatas.
	*/

	// Loop utama untuk membaca input dan mengirimkannya ke server secara terus menerus
	for {
		//Membaca input dari user
		message, err := localReader.ReadString('\n')
		if err != nil {
			break
		}

		// Menangani kondisi jika user memasukkan "/quit"
		if strings.TrimSpace(message) == "/quit" {
			fmt.Println("Goodbye!")
			break
		}

		// Mengirimkan pesan dari user ke server
		_, err = conn.Write([]byte(message))
		if err != nil {
			fmt.Println("Error sending message to server.")
			break
		}

		fmt.Print(prompt)
	}
}
