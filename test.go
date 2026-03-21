package main

import "net"

func main() {
	
	net.Conn
	net.Listen()

	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}

		hub.register <- conn
		go handleClient(conn, hub)
	}
}