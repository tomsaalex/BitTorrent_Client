package torrentclient

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
)

type connBootstrapInfo struct {
	conn          net.Conn
	protocolStr   []byte
	reservedBytes []byte
}

func handleIncomingHandshake(conn net.Conn, servedTorrentRequest chan<- []byte, servedTorrentReply chan (chan<- connBootstrapInfo)) error {
	protocolStrLen := make([]byte, 1)
	_, err := conn.Read(protocolStrLen)

	if err != nil {
		return &PeerCommunicationError{Message: "Couldn't read pstr length while receiving handshake."}
	}

	protocolStr := make([]byte, protocolStrLen[0])

	_, err = io.ReadFull(conn, protocolStr)

	if err != nil {
		return &PeerCommunicationError{Message: "Couldn't read pstr while receiving handshake."}
	}

	if string(protocolStr) != "BitTorrent protocol" {
		return &PeerCommunicationError{Message: "Received handshake referenced unknown protocol: " + string(protocolStr)}
	}

	reserved := make([]byte, 8)
	_, err = io.ReadFull(conn, reserved)

	if err != nil {
		return &PeerCommunicationError{Message: "Couldn't read the 8 reserved bytes while receiving handshake."}
	}

	infohash := make([]byte, 20)

	_, err = io.ReadFull(conn, infohash)

	if err != nil {
		return &PeerCommunicationError{Message: "Couldn't read the infohash while receiving handshake."}
	}

	servedTorrentRequest <- infohash
	connForwarder := <-servedTorrentReply

	if connForwarder == nil {
		return &PeerCommunicationError{Message: "Received handshake contained unknown infohash: " + string(infohash)}
	}

	connForwarder <- connBootstrapInfo{
		conn:          conn,
		protocolStr:   protocolStr,
		reservedBytes: reserved,
	}

	return nil
}

func handleIncomingConnections(servedTorrentRequest chan []byte, servedTorrentReply chan (chan<- connBootstrapInfo)) {
	const port = 63999 // TODO: Either let the user set one or at least first iterate through the usual ones. Sync with trackerManager.

	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port)) // Ensure this matches your announce port
	if err != nil {
		// TODO: Handle this error better
		panic(err)
	}
	slog.LogAttrs(
		context.Background(),
		slog.LevelInfo,
		"Listening for incoming connections",
		slog.Int("port", port),
	)
	for {
		conn, err := ln.Accept()
		if err != nil {
			// TODO: Handle this error better
			panic(err)
		}

		fmt.Println(conn.LocalAddr().String())
		fmt.Println(conn.RemoteAddr().String())
		slog.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"Incoming connection detected",
			slog.String("remoteAddress", conn.RemoteAddr().String()),
		)

		err = handleIncomingHandshake(conn, servedTorrentRequest, servedTorrentReply)
		if err != nil {
			conn.Close()

			slog.LogAttrs(
				context.Background(),
				slog.LevelError,
				"Connection dropped",
				slog.String("Reason", err.Error()),
			)
		}
	}
}
