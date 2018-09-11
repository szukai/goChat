# goChat
tcp gochat client demo

A tcp text chat server made while learning Go.

To run the server:

```go run gochat.go```

Users can join via tcp on port 6000. Commands include:

/rooms - display rooms

/join <roomname> - Join a room, a room will be created if none exist

/quit - disconnect

