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

// listRooms mengembalikan string list room yang tersedia.
// Fungsi ini sekarang lebih generik untuk command /rooms.
func listRooms() string {
	result := "\nRoom yang tersedia:\n"
	for _, room := range rooms {
		result += fmt.Sprintf("- %s\n", room)
	}
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
		if isNameTaken(name) {
			fmt.Fprintf(conn, "Nama tidak tersedia, masukkan nama yang berbeda:\n")
		} else {
			break
		}
	}

	// Buat client baru dan secara otomatis tempatkan di room "general".
	client := &Client{
		conn: conn,
		name: name,
		room: "general", // MODIFIKASI: Room default adalah "general".
	}

	mutex.Lock()
	clients[conn] = client
	mutex.Unlock()

	// Beri tahu room "general" bahwa ada user baru yang bergabung.
	fmt.Printf("%s bergabung ke room 'general'\n", name)
	broadcastMessage(fmt.Sprintf("\n%s baru saja bergabung.\n", name), conn, "general")

	// Kirim pesan selamat datang ke client baru.
	fmt.Fprintf(conn, "\nSelamat datang, %s! Anda telah ditempatkan di room 'general'.\n", name)
	fmt.Fprintf(conn, "Ketik /rooms untuk melihat room lain, /join <room> untuk pindah, atau /leave untuk keluar dari room.\n\n")

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
			command := strings.Fields(message)
			switch command[0] {
			case "/rooms":
				fmt.Fprintf(conn, listRooms())
			case "/join":
				if len(command) > 1 {
					roomToJoin := command[1]
					validRoom := false
					for _, r := range rooms {
						if r == roomToJoin {
							validRoom = true
							break
						}
					}

					if validRoom {
						// Beri tahu room lama bahwa user telah keluar.
						if client.room != "" {
							broadcastMessage(fmt.Sprintf("%s telah meninggalkan room ini.\n", name), conn, client.room)
						}
						// Perbarui room client dan beri tahu room baru.
						client.room = roomToJoin
						fmt.Fprintf(conn, "Berhasil bergabung ke '%s'.\n", client.room)
						broadcastMessage(fmt.Sprintf("%s baru saja bergabung.\n", name), conn, client.room)
						fmt.Printf("%s bergabung ke room %s\n", name, client.room)
					} else {
						fmt.Fprintf(conn, "Nama room tidak valid. Ketik /rooms untuk melihat daftar.\n")
					}
				} else {
					fmt.Fprintf(conn, "Penggunaan: /join <nama_room>\n")
				}
			case "/leave":
				if client.room != "" {
					fmt.Fprintf(conn, "Anda meninggalkan '%s'.\n", client.room)
					broadcastMessage(fmt.Sprintf("%s telah meninggalkan room ini.\n", name), conn, client.room)
					fmt.Printf("%s meninggalkan room %s\n", name, client.room)
					client.room = "" // Client sekarang berada di "lobby"
				} else {
					fmt.Fprintf(conn, "Anda tidak berada di dalam room manapun.\n")
				}
			default:
				fmt.Fprintf(conn, "Command tidak dikenal: %s\n", command[0])
			}
		} else {
			// Menangani pesan chat biasa
			if message != "" {
				if client.room != "" {
					broadcastMessage(fmt.Sprintf("[%s] %s > %s\n", client.room, name, message), conn, client.room)
				} else {
					fmt.Fprintf(conn, "Anda tidak berada di dalam room. Gunakan /join <room> untuk memulai chat.\n")
				}
			}
		}
	}

	// Membersihkan saat client terputus.
	mutex.Lock()
	delete(clients, conn)
	mutex.Unlock()

	// Beri tahu room jika client berada di dalamnya saat terputus.
	if client.room != "" {
		fmt.Printf("%s telah terputus.\n", name)
		broadcastMessage(fmt.Sprintf("%s telah meninggalkan chat.\n", name), conn, client.room)
	}
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
