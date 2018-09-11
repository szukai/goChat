package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

// ChatRoom structure - keeps track of each chatroom
type ChatRoom struct {
	name        string
	users       map[net.Conn]*User // User connections
	newUser     chan net.Conn      // new user
	removedUser chan net.Conn      // user removed
	messages    chan string        // message channel
}

// User structure - keeps track of user
type User struct {
	// connection chan net.Conn // User connection
	name string    // user name
	room *ChatRoom // chatroom occupied
}

// NewChatRoom - Create a new room
func NewChatRoom(groupName string) *ChatRoom {
	chatRoom := &ChatRoom{
		name:        groupName,
		users:       make(map[net.Conn]*User),
		newUser:     make(chan net.Conn),
		removedUser: make(chan net.Conn),
		messages:    make(chan string),
	}

	// Lobby uses slightly different logic
	if chatRoom.name != "lobby" {
		go chatRoom.run()
	}

	return chatRoom
}

// run
func (chatRoom *ChatRoom) run() {
	for {
		select {
		// receive
		case conn := <-chatRoom.newUser:
			userName := chatRoom.users[conn].name
			io.WriteString(conn, fmt.Sprintf("Entering %s...\n[%s]> ", chatRoom.name, chatRoom.name))
			log.Printf("User (%s) joined group: %s", userName, chatRoom.name)
			go func(userName string) {
				chatRoom.messages <- fmt.Sprintf("Welcome %s!\n", userName)
			}(userName)

		case message := <-chatRoom.messages:
			for conn := range chatRoom.users {
				go func(conn net.Conn, message string) {
					_, err := io.WriteString(conn, message)
					if err != nil {
						chatRoom.removedUser <- conn
					}
					io.WriteString(conn, fmt.Sprintf("\n[%s]> ", chatRoom.name))
				}(conn, message)
			}

		case conn := <-chatRoom.removedUser: // Handle dead users
			userName := chatRoom.users[conn].name
			io.WriteString(conn, fmt.Sprintf("Leaving %s...\n", chatRoom.name))
			log.Printf("User (%s) left group: %s", userName, chatRoom.name)
			go func(conn net.Conn, userName string) {
				delete(chatRoom.users, conn)
				chatRoom.messages <- fmt.Sprintf("%s has left.\n", userName)
			}(conn, userName)
		}
	}
}

func main() {
	const PORT = ":6000"

	chatrooms := make(map[string]*ChatRoom)
	// Initialize with lobby
	lobby := NewChatRoom("lobby")
	chatrooms["lobby"] = lobby

	users := lobby.users
	newUser := lobby.newUser
	removedUser := lobby.removedUser
	messages := lobby.messages
	// newConnection := lobby.newConnection

	newConnection := make(chan net.Conn) // Handle new connection
	quitUser := make(chan net.Conn)

	// Net connection
	server, err := net.Listen("tcp", PORT)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	log.Print("Starting goChat...")
	// Server connections
	go func() {
		for {
			conn, err := server.Accept()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			newConnection <- conn // Send to handle new user
			log.Print("new connection")
		}
	}()
	for {
		select {
		// Ask user for name and information
		case conn := <-newConnection:

			go func(conn net.Conn) {
				//Lobby by default
				reader := bufio.NewReader(conn)
				io.WriteString(conn, "Welcome to goChat\nLogin Name: ")
				userName, _ := reader.ReadString('\n')
				userName = strings.Trim(userName, "\r\n") // Trim
				user := &User{
					name: userName,
					room: lobby,
				}
				log.Printf("User logged in: %s", userName)
				io.WriteString(conn, "[lobby]> ")
				users[conn] = user // Add connection
				newUser <- conn    // Add user to pool
			}(conn)

		case conn := <-newUser:
			// send
			go func(conn net.Conn, user User) {
				userName := user.name
				reader := bufio.NewReader(conn)

				for {
					// io.WriteString(conn, "> ")
					newMessage, err := reader.ReadString('\n')
					newMessage = strings.Trim(newMessage, "\r\n")
					if err != nil {
						break
					}
					// Attempt to get commands
					commands := strings.Split(newMessage, " ")
					switch commands[0] {
					case "/rooms":
						io.WriteString(conn, "Active rooms:\n")
						for name, room := range chatrooms {
							io.WriteString(conn, fmt.Sprintf("/%s (%d)\n", name, len(room.users)))
						}
						io.WriteString(conn, fmt.Sprintf("[%s]> ", user.room.name))
					// same as join lobby
					case "/leave":
						lobby.users[conn] = &user
						user.room.removedUser <- conn
						user.room = lobby
						lobby.newUser <- conn

					case "/quit":
						io.WriteString(conn, "Quitting...\n")
						quitUser <- conn
					// Swap rooms
					case "/join":
						if len(commands) == 2 {
							nChatRoomName := commands[1]
							if nChatRoomName == user.room.name {
								io.WriteString(conn, "You are already in the room.")
							} else {
								if nChatRoom, ok := chatrooms[nChatRoomName]; ok { //Room exists
									nChatRoom.users[conn] = &user
									user.room.removedUser <- conn
									//delete(user.room.users, conn)
									user.room = nChatRoom
									nChatRoom.newUser <- conn
								} else { // New Room
									nChatRoom = NewChatRoom(nChatRoomName)
									chatrooms[nChatRoomName] = nChatRoom
									nChatRoom.users[conn] = &user
									user.room.removedUser <- conn
									//delete(user.room.users, conn)
									user.room = nChatRoom
									nChatRoom.newUser <- conn
									log.Printf("New Chatroom created: %s", nChatRoomName)
								}
							}
						} else {
							io.WriteString(conn, "Incorrect number of arguments.\n")
							break
						}
					// Not command, send message
					default:
						user.room.messages <- fmt.Sprintf("%s> %s", userName, newMessage)
					}
				}
				removedUser <- conn // If error occurs, connection has been terminated
				messages <- fmt.Sprintf("%s has quit.\n", userName)
				log.Printf("%s disconnected", userName)
			}(conn, *users[conn])

		// receive
		case message := <-messages: // If message recieved from any user
			for conn := range users { // Send to all users
				go func(conn net.Conn, message string) { // Write to all user connections
					_, err := io.WriteString(conn, message)
					if err != nil {
						removedUser <- conn
					}
					io.WriteString(conn, "\n[lobby]> ")
				}(conn, message)
			}
			// log.Printf("lobby message: %s", message)

		case conn := <-removedUser:
			delete(users, conn)
		case conn := <-quitUser:
			delete(users, conn)
			conn.Close()
		}
	}
	// end-Loop
}
