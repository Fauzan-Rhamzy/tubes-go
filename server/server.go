package main

/*
Sumber:
1. kode Contoh Client-Server dari echo-example.zip
2. claude.ai sebagai sumner pembelajaran
3. https://medium.com/@iggeehu/learning-go-by-writing-a-simple-tcp-server-d8ed260f67ac sebagai sumner pembelajaran
*/

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

// Client merepresentasikan user yang terkoneksi.
type Client struct {
	conn net.Conn
	name string
	room string
}

var (
	// clients adalah map untuk menyimpan semua client yang terkoneksi.
	clients = make(map[net.Conn]*Client)
	// rooms adalah slice yang berisi room chat yang tersedia.
	rooms = []string{"general", "games", "hobby", "study", "chill", "qna"}
	// mutex digunakan untuk akses yang aman ke sumber daya yang dipakai bersama.
	mutex = &sync.Mutex{}
)

// listRooms mengembalikan string list room yang tersedia untuk dipilih.
func listRoomsForSelection() string {
	result := "\nRoom yang tersedia:\n"
	for _, room := range rooms {
		result += fmt.Sprintf("- %s\n", room)
	}
	result += "Pilih room untuk bergabung:\n"
	return result
}

// broadcastMessage mengirim pesan ke semua client di room tertentu, kecuali pengirim.
func broadcastMessage(message string, sender net.Conn, room string) {
	mutex.Lock()
	defer mutex.Unlock()

	for conn, client := range clients {
		if conn != sender && client.room == room {
			fmt.Fprint(conn, message)
		}
	}
}

// broadcastToAll mengirim pesan ke semua client di server, kecuali pengirim.
func broadcastToAll(message string, sender net.Conn) {
	mutex.Lock()
	defer mutex.Unlock()

	for conn := range clients {
		if conn != sender {
			fmt.Fprint(conn, message)
		}
	}
}

// isNameTaken mengecek apakah sebuah username sudah digunakan.
func isNameTaken(name string) bool {
	mutex.Lock()
	defer mutex.Unlock()

	lowerCaseName := strings.ToLower(name)
	for _, client := range clients {
		if strings.ToLower(client.name) == lowerCaseName {
			return true
		}
	}
	return false
}

// handleCommand memproses semua command yang dikirim client
func handleCommand(client *Client, conn net.Conn, message string) {
	command := strings.Fields(message)
	if len(command) == 0 {
		return
	}

	switch command[0] {
	case "/rooms":
		handleRoomsCommand(conn)
	case "/join":
		handleJoinCommand(client, conn, command)
	case "/leave":
		handleLeaveCommand(client, conn)
	case "/help":
		handleHelpCommand(conn)
	default:
		fmt.Fprintf(conn, "[SERVER] Command tidak dikenal: %s. Ketik /help untuk bantuan.\n", command[0])
	}
}

// handleRoomsCommand menampilkan daftar room yang tersedia
func handleRoomsCommand(conn net.Conn) {
	fmt.Fprint(conn, listRoomsForSelection())
}

// handleJoinCommand menangani perpindahan user ke room lain
func handleJoinCommand(client *Client, conn net.Conn, command []string) {
	if len(command) < 2 {
		fmt.Fprintf(conn, "[SERVER] Penggunaan: /join <nama_room>\n")
		return
	}

	roomToJoin := command[1]

	// Validasi nama room
	validRoom := false
	for _, r := range rooms {
		if r == roomToJoin {
			validRoom = true
			break
		}
	}

	if !validRoom {
		fmt.Fprintf(conn, "[SERVER] Nama room tidak valid. Ketik /rooms untuk melihat daftar.\n")
		return
	}

	// Beri tahu room lama bahwa user telah keluar
	if client.room != "" {
		broadcastMessage(fmt.Sprintf("[SERVER] %s telah meninggalkan room ini.\n", client.name), conn, client.room)
	}

	// Perbarui room client dan beri tahu room baru
	client.room = roomToJoin
	fmt.Fprintf(conn, "[SERVER] Berhasil bergabung ke '%s'.\n", client.room)
	broadcastMessage(fmt.Sprintf("[SERVER] %s baru saja bergabung.\n", client.name), conn, client.room)
	fmt.Printf("[SERVER] %s bergabung ke room %s\n", client.name, client.room)
}

// handleLeaveCommand menangani user keluar dari room
func handleLeaveCommand(client *Client, conn net.Conn) {
	if client.room == "" {
		fmt.Fprintf(conn, "[SERVER] Anda tidak berada di dalam room manapun.\n")
		return
	}

	fmt.Fprintf(conn, "[SERVER] Anda meninggalkan '%s'.\n", client.room)
	broadcastMessage(fmt.Sprintf("[SERVER] %s telah meninggalkan room ini.\n", client.name), conn, client.room)
	fmt.Printf("[SERVER] %s meninggalkan room %s\n", client.name, client.room)
	client.room = "" // Client sekarang berada di "lobby"
}

// handleHelpCommand menampilkan bantuan command
func handleHelpCommand(conn net.Conn) {
	help := `
[SERVER] Command yang tersedia:
  /rooms - Melihat daftar room yang tersedia
  /join <room> - Bergabung ke room tertentu
  /leave - Keluar dari room saat ini
  /help - Menampilkan bantuan ini
  /quit - Keluar dari server (hanya di client)
`
	fmt.Fprintf(conn, "%s\n", help)
}

// handleClient mengatur seluruh siklus koneksi client.
func handleClient(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Dapatkan username yang unik dari client.
	var name string
	for {
		nameInput, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Gagal membaca nama: %v\n", err)
			return
		}
		name = strings.TrimSpace(nameInput)

		// Validasi username tidak kosong
		if name == "" {
			fmt.Fprintf(conn, "Username tidak boleh kosong, silahkan masukkan username.\n")
			continue
		}

		if isNameTaken(name) {
			fmt.Fprintf(conn, "Nama tidak tersedia, masukkan nama yang berbeda.\n")
			continue
		}

		// Username valid dan tidak diambil
		break
	}

	// Beri tahu semua client bahwa ada koneksi baru.
	fmt.Printf("%s telah terhubung ke server.\n", name)
	broadcastToAll(fmt.Sprintf("\n[SERVER] %s telah terhubung ke server.\n", name), conn)

	// Buat client baru, room awalnya kosong.
	client := &Client{
		conn: conn,
		name: name,
		room: "",
	}

	mutex.Lock()
	clients[conn] = client
	mutex.Unlock()

	// Loop untuk meminta client memilih room.
roomSelect:
	for {
		fmt.Fprint(conn, listRoomsForSelection()) // Kirim daftar room ke client

		roomChoice, err := reader.ReadString('\n')
		if err != nil {
			break // Keluar jika ada error
		}
		roomChoice = strings.TrimSpace(roomChoice)

		// Validasi pilihan room
		validRoom := false
		for _, r := range rooms {
			if strings.EqualFold(r, roomChoice) {
				client.room = r
				validRoom = true
				break
			}
		}

		if validRoom {
			fmt.Fprintf(conn, "[SERVER] \nSelamat datang, %s! Anda berhasil bergabung ke room '%s'.\n", name, client.room)
			fmt.Fprintf(conn, "Ketik /help untuk melihat daftar command yang tersedia.\n\n")

			// Beri tahu room bahwa ada user baru yang bergabung.
			broadcastMessage(fmt.Sprintf("[SERVER] %s baru saja bergabung dengan room ini.\n", name), conn, client.room)
			fmt.Printf("[SERVER] %s bergabung ke room '%s'\n", name, client.room)
			break roomSelect // Keluar dari loop pemilihan room
		} else {
			fmt.Fprintf(conn, "[SERVER] Nama room tidak valid, silakan coba lagi.\n")
		}
	}

	// Loop utama untuk memproses pesan dari client.
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			// Jika ada error, kemungkinan client terputus. Hentikan loop.
			break
		}

		message = strings.TrimSpace(message)

		if strings.HasPrefix(message, "/") {
			// Menangani command
			handleCommand(client, conn, message)
		} else {
			// Menangani pesan chat biasa
			if message != "" {
				if client.room != "" {
					broadcastMessage(fmt.Sprintf("[%s] %s > %s\n", client.room, name, message), conn, client.room)
				} else {
					fmt.Fprintf(conn, "[SERVER] Anda tidak berada di dalam room. Gunakan /join <room> untuk memulai chat.\n")
				}
			}
		}
	}

	// Membersihkan saat client terputus.
	mutex.Lock()
	// Hanya hapus client jika masih ada di map
	if _, ok := clients[conn]; ok {
		delete(clients, conn)
	}
	mutex.Unlock()

	// Beri tahu semua orang di server bahwa client ini telah terputus.
	fmt.Printf("[SERVER] %s telah terputus.\n", name)
	broadcastToAll(fmt.Sprintf("\n[SERVER] %s telah terputus.\n", name), conn)
}

func main() {
	ln, err := net.Listen("tcp", ":9090")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal melakukan listen: %v\n", err)
		os.Exit(1)
	}
	defer ln.Close()

	fmt.Println("Server Chat berjalan di port 9090")
	fmt.Println("Room yang tersedia:", rooms)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Gagal menerima koneksi: %v\n.", err)
			continue
		}
		go handleClient(conn)
	}
}
