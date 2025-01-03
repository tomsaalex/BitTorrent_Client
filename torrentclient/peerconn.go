package torrentclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"strconv"
	"time"

	generalerrors "github.com/tomsaalex/BitTorrent_Client/GeneralErrors"
	"github.com/tomsaalex/BitTorrent_Client/customdatatypes"
)

// A bit smaller than 2 minutes to make sure the connection survives
const KEEP_ALIVE_TIME = time.Second * 110

const CONNECTION_SPEED_UPDATE_TIME = time.Second

type StateUpdateType int

const (
	Choked StateUpdateType = iota
	Unchoked
	Interested
	NotInterested
)

type connData struct {
	amChoking      bool
	amInterested   bool
	peerChoking    bool
	peerInterested bool
}

type connMessage struct {
	peerConn peerConnection
	peerMsg  peerMessage
}

type peerConnection struct {
	otherPeer  peer
	connection net.Conn
	pieceCount int

	remotePeerBitfield customdatatypes.FixedSizeBitfield

	dataTransferSpeed         int
	dataTransferSpeedSnapshot int // To have a stable value while sorting

	input          chan peerMessage
	output         chan connMessage
	assemblerInput chan pieceMessage

	connDataRequests chan bool
	connDataReplies  chan connData

	pieceCheck  chan int
	pieceStatus chan bool

	speedRequests chan bool
	speedReplies  chan int
}

func newPeerConnection(peer peer, td TorrentData, output chan connMessage, assemblerInput chan pieceMessage) (peerConnection, error) {
	connectAddress := peer.ip + ":" + strconv.Itoa(int(peer.port))

	conn, err := net.Dial("tcp", connectAddress)

	if err != nil {
		return peerConnection{}, &PeerConnectionError{Message: "Failed to establish peer connection", InvolvedPeer: peer}
	}
	input := make(chan peerMessage)

	cdRequests := make(chan bool)
	cdReplies := make(chan connData)
	pieceCheck := make(chan int)
	pieceStatus := make(chan bool)
	speedRequests := make(chan bool)
	speedReplies := make(chan int)

	peerConnection := peerConnection{otherPeer: peer, connection: conn, pieceCount: len(td.PieceHashes), input: input, output: output, assemblerInput: assemblerInput, connDataRequests: cdRequests, connDataReplies: cdReplies, pieceCheck: pieceCheck, pieceStatus: pieceStatus, speedRequests: speedRequests, speedReplies: speedReplies}
	return peerConnection, nil
}

func (pc *peerConnection) createRemotePeerBitfield() (*customdatatypes.FixedSizeBitfield, error) {
	remotePeerBitfield, err := customdatatypes.NewFixedSizeBitfield(pc.pieceCount)
	if err != nil {
		_, isTypeInitErr := err.(*generalerrors.TypeInitializationError)

		if isTypeInitErr {
			return nil, &MalformedTorrentError{Message: "Couldn't initialize bitfield for torrent. Number of pieces invalid."}
		}
	}

	return remotePeerBitfield, nil
}

func addDataPoint(dataPoints []int, newPoint int) []int {
	return append(dataPoints[1:], newPoint)
}

func calcConnectionSpeed(dataPoints []int) int {
	rollingAverage := 0
	for _, dp := range dataPoints {
		rollingAverage += dp
	}

	// We're working with byte counts here, so we can afford to round without losing anything significant
	return rollingAverage / len(dataPoints)
}

func (pc *peerConnection) launchConnectionRoutines() {
	forwarderStateUpdater := make(chan StateUpdateType)
	receiverStateUpdater := make(chan StateUpdateType)

	receiverPeerIndexUpdater := make(chan int)
	downloadDataRateUpdater := make(chan int)

	peerBitfieldUpdater := make(chan []byte)

	go pc.connDataManager(forwarderStateUpdater, receiverStateUpdater, receiverPeerIndexUpdater, downloadDataRateUpdater, peerBitfieldUpdater)
	go pc.forwarderController(forwarderStateUpdater)
	go pc.receiverController(receiverStateUpdater, receiverPeerIndexUpdater, downloadDataRateUpdater, peerBitfieldUpdater)
}

func (pc *peerConnection) forwarderController(stateUpdater chan<- StateUpdateType) {
	toForwarder := make(chan peerMessage)

	go pc.forwarderRoutine(toForwarder)

	for {
		select {
		case rawMessage := <-pc.input:
			toForwarder <- rawMessage
			// TODO: Should really quantify the amount of data sent in the connDataManager routine
			switch rawMessage.(type) {
			case chokeMessage:
				stateUpdater <- Choked
			case unchokeMessage:
				stateUpdater <- Unchoked
			case interestedMessage:
				stateUpdater <- Interested
			case notInterestedMessage:
				stateUpdater <- NotInterested
			}
		}
	}
}

func (pc *peerConnection) receiverController(stateUpdater chan<- StateUpdateType, peerIndexUpdate chan<- int, dataRateUpdate chan<- int, peerBitfieldUpdate chan<- []byte) {
	receiverOutput := make(chan peerMessage)

	go pc.receiverRoutine(receiverOutput)

	for {
		select {
		case rawMessage := <-receiverOutput:
			/* TODO: Perhaps I shouldn't be accepting all traffic, but some traffic needs to go through still
			 if cd.amChoking {
				continue
			} */
			switch peerMessage := rawMessage.(type) {
			case chokeMessage:
				stateUpdater <- Choked
			case unchokeMessage:
				stateUpdater <- Unchoked
			case interestedMessage:
				stateUpdater <- Interested
			case notInterestedMessage:
				stateUpdater <- NotInterested
			case haveMessage:
				peerIndexUpdate <- peerMessage.pieceIndex
			case pieceMessage:
				pc.assemblerInput <- peerMessage
				pc.output <- connMessage{peerMsg: peerMessage, peerConn: *pc}
				dataRateUpdate <- len(peerMessage.block)
			case requestMessage:
				pc.output <- connMessage{peerMsg: peerMessage, peerConn: *pc}
			case cancelMessage:
				pc.output <- connMessage{peerMsg: peerMessage, peerConn: *pc}
			case bitfieldMessage:
				peerBitfieldUpdate <- peerMessage.bitfield
			}
		}
	}
}

func (pc *peerConnection) connDataManager(forwarderStateUpdate <-chan StateUpdateType, receiverStateUpdate <-chan StateUpdateType, peerIndexUpdate <-chan int, receiverDataRateUpdate <-chan int, peerBitfieldUpdate <-chan []byte) {
	downloadDataCounter := 0
	downloadRates := make([]int, 100)

	dataRateTicker := time.NewTicker(CONNECTION_SPEED_UPDATE_TIME)

	cd := connData{amChoking: true, amInterested: false, peerChoking: true, peerInterested: false}
	peerBitfield, err := pc.createRemotePeerBitfield()

	if err != nil {
		// TODO: Handle this more elegantly somehow. It's technically an impossible error, but we still don't want to crash.
		panic(err)
	}

	for {
		select {
		case updateType := <-forwarderStateUpdate:
			switch updateType {
			case Choked:
				cd.amChoking = true
			case Unchoked:
				cd.amChoking = false
			case Interested:
				cd.amInterested = true
			case NotInterested:
				cd.amInterested = false
			}
		case updateType := <-receiverStateUpdate:
			switch updateType {
			case Choked:
				cd.peerChoking = true
			case Unchoked:
				cd.peerChoking = false
			case Interested:
				cd.peerInterested = true
			case NotInterested:
				cd.peerInterested = false
			}
		case newHas := <-peerIndexUpdate:
			peerBitfield.SetBit(newHas)
		case dataReport := <-receiverDataRateUpdate:
			downloadDataCounter += dataReport
		case <-pc.connDataRequests:
			pc.connDataReplies <- cd
		case index := <-pc.pieceCheck:
			pieceCheckResult, _ := peerBitfield.IsSet(index)
			pc.pieceStatus <- pieceCheckResult
		case <-pc.speedRequests:
			pc.speedReplies <- pc.dataTransferSpeed
		case <-dataRateTicker.C:
			downloadRates = addDataPoint(downloadRates, downloadDataCounter)
			pc.dataTransferSpeed = calcConnectionSpeed(downloadRates)
			downloadDataCounter = 0
		case bitfield := <-peerBitfieldUpdate:
			// TODO: I think we need to check if any of the extra bits are set and drop the connection if so
			err := peerBitfield.ImportBitfield(bitfield)
			if err != nil {
				// TODO: DROP CONNECTION
			}
		}
	}
}

func (pc *peerConnection) connectionRoutine() {
	// TODO: Maybe make the channels one way, but to do that every channel operation must have its separate function,
	// and you need to stick to the convention. Not sure if that's better

	receiverOutput := make(chan peerMessage)
	toForwarder := make(chan peerMessage)

	dataCounter := 0
	downloadRates := make([]int, 100)

	dataRateTicker := time.NewTicker(CONNECTION_SPEED_UPDATE_TIME)

	cd := connData{amChoking: true, amInterested: false, peerChoking: true, peerInterested: false}
	peerBitfield, err := pc.createRemotePeerBitfield()

	if err != nil {
		// TODO: Handle this more elegantly somehow. It's technically an impossible error, but we still don't want to crash.
		panic(err)
	}

	go pc.forwarderRoutine(toForwarder)
	go pc.receiverRoutine(receiverOutput)

	for {
		select {
		case rawMessage := <-receiverOutput:
			/* TODO: Perhaps I shouldn't be accepting any traffic, but some traffic needs to go through still
			 if cd.amChoking {
				continue
			} */
			switch peerMessage := rawMessage.(type) {
			case chokeMessage:
				cd.peerChoking = true
			case unchokeMessage:
				cd.peerChoking = false
			case interestedMessage:
				cd.peerInterested = true
			case notInterestedMessage:
				cd.peerInterested = false
			case haveMessage:
				peerBitfield.SetBit(peerMessage.pieceIndex)
			case pieceMessage:
				pc.output <- connMessage{peerMsg: peerMessage, peerConn: *pc}
				dataCounter += len(peerMessage.block)
			case requestMessage:
				pc.output <- connMessage{peerMsg: peerMessage, peerConn: *pc}
			case cancelMessage:
				pc.output <- connMessage{peerMsg: peerMessage, peerConn: *pc}
			case bitfieldMessage:
				// TODO: I think we need to check if any of the extra bits are set and drop the connection if so
				err := peerBitfield.ImportBitfield(peerMessage.bitfield)
				if err != nil {
					// TODO: DROP CONNECTION
				}
			}
		case rawMessage := <-pc.input:
			toForwarder <- rawMessage
			switch rawMessage.(type) {
			case chokeMessage:
				cd.amChoking = true
			case unchokeMessage:
				cd.amChoking = false
			case interestedMessage:
				cd.amInterested = true
			case notInterestedMessage:
				cd.amInterested = false
			}
		case <-pc.connDataRequests:
			pc.connDataReplies <- cd
		case index := <-pc.pieceCheck:
			pieceCheckResult, _ := peerBitfield.IsSet(index)
			pc.pieceStatus <- pieceCheckResult
		case <-pc.speedRequests:
			pc.speedReplies <- pc.dataTransferSpeed
		case <-dataRateTicker.C:
			downloadRates = addDataPoint(downloadRates, dataCounter)
			pc.dataTransferSpeed = calcConnectionSpeed(downloadRates)
			dataCounter = 0
		}
	}
}

func (pc *peerConnection) forwarderRoutine(msgInput <-chan peerMessage) {
	keepAliveTicker := time.NewTicker(KEEP_ALIVE_TIME)

	for {
		select {
		case rawMsg := <-msgInput:
			switch peerMessage := rawMsg.(type) {
			case chokeMessage:
				pc.sendChoke()
				slog.LogAttrs(
					context.Background(),
					slog.LevelInfo,
					"Sent choke",
					slog.String("peerIP", pc.otherPeer.ip),
					slog.Int("PeerPort", int(pc.otherPeer.port)),
				)
			case unchokeMessage:
				pc.sendUnchoke()
				slog.LogAttrs(
					context.Background(),
					slog.LevelInfo,
					"Sent unchoke",
					slog.String("peerIP", pc.otherPeer.ip),
					slog.Int("PeerPort", int(pc.otherPeer.port)),
				)
			case interestedMessage:
				pc.sendInterested()
				slog.LogAttrs(
					context.Background(),
					slog.LevelInfo,
					"Sent interested",
					slog.String("peerIP", pc.otherPeer.ip),
					slog.Int("PeerPort", int(pc.otherPeer.port)),
				)
			case notInterestedMessage:
				pc.sendNotInterested()
				slog.LogAttrs(
					context.Background(),
					slog.LevelInfo,
					"Sent not interested",
					slog.String("peerIP", pc.otherPeer.ip),
					slog.Int("PeerPort", int(pc.otherPeer.port)),
				)
			case haveMessage:
				pc.sendHave(peerMessage.pieceIndex)
				slog.LogAttrs(
					context.Background(),
					slog.LevelInfo,
					"Sent have",
					slog.Int("PieceIndex", peerMessage.pieceIndex),
					slog.String("peerIP", pc.otherPeer.ip),
					slog.Int("PeerPort", int(pc.otherPeer.port)),
				)
			case pieceMessage:
				index := peerMessage.index
				begin := peerMessage.begin
				block := peerMessage.block
				pc.sendPiece(index, begin, block)
				slog.LogAttrs(
					context.Background(),
					slog.LevelInfo,
					"Sent piece block",
					slog.Int("PieceIndex", index),
					slog.Int("BlockOffset", begin),
					slog.String("peerIP", pc.otherPeer.ip),
					slog.Int("PeerPort", int(pc.otherPeer.port)),
				)
			case requestMessage:
				index := peerMessage.index
				begin := peerMessage.begin
				length := peerMessage.length
				pc.sendRequest(index, begin, length)
				slog.LogAttrs(
					context.Background(),
					slog.LevelInfo,
					"Sent block request",
					slog.Int("PieceIndex", index),
					slog.Int("BlockOffset", begin),
					slog.String("peerIP", pc.otherPeer.ip),
					slog.Int("PeerPort", int(pc.otherPeer.port)),
				)
			case cancelMessage:
				index := peerMessage.index
				begin := peerMessage.begin
				length := peerMessage.length
				pc.sendCancel(index, begin, length)
				slog.LogAttrs(
					context.Background(),
					slog.LevelInfo,
					"Sent cancel request",
					slog.Int("PieceIndex", index),
					slog.Int("BlockOffset", begin),
					slog.String("peerIP", pc.otherPeer.ip),
					slog.Int("PeerPort", int(pc.otherPeer.port)),
				)
			}
		case <-keepAliveTicker.C:
			pc.sendKeepAlive()
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Sent keep alive",
				slog.String("peerIP", pc.otherPeer.ip),
				slog.Int("PeerPort", int(pc.otherPeer.port)),
			)
		}
	}
}

func (pc *peerConnection) receiverRoutine(msgOutput chan<- peerMessage) {
	buffcon := bufio.NewReader(pc.connection)

	for {
		// TODO: Perhaps we shouldn't even be accepting data from choked peers?
		peerMessage, err := pc.receiveMessage(buffcon)
		if err != nil {
			// TODO: Handle this more nicely, though idk how, cause this is in a goroutine
			panic(err)
		}
		msgOutput <- peerMessage
	}
}

func (pc *peerConnection) receiveMessage(buffcon *bufio.Reader) (peerMessage, error) {
	lenBuf := make([]byte, 4)
	var err error

	for i := 0; i <= 3; i++ {
		lenBuf[i], err = buffcon.ReadByte()
		if err != nil {
			return nil, &PeerCommunicationError{Message: "Couldn't read message length."}
		}
	}

	msgLen := int(binary.BigEndian.Uint32(lenBuf))
	if msgLen == 0 {
		return keepAliveMessage{}, nil
	}

	msgIDByte, err := buffcon.ReadByte()
	msgID := int(msgIDByte)
	if err != nil {
		return nil, &PeerCommunicationError{Message: "Couldn't read message ID."}
	}

	// Enough to handle a piece message and a big bitfield
	// Just a pessimistic estimate
	msgBuf := make([]byte, 20000)

	_, err = io.ReadFull(buffcon, msgBuf[:msgLen-1])

	if err != nil {
		return nil, &PeerCommunicationError{Message: "Couldn't read message payload."}
	}

	payloadLen := msgLen - 1

	switch msgID {
	case CHOKE_MESSAGE:
		slog.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"Received choke",
			slog.String("peerIP", pc.otherPeer.ip),
			slog.Int("PeerPort", int(pc.otherPeer.port)),
		)
		return chokeMessage{}, nil
	case UNCHOKE_MESSAGE:
		slog.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"Received unchoke",
			slog.String("peerIP", pc.otherPeer.ip),
			slog.Int("PeerPort", int(pc.otherPeer.port)),
		)
		return unchokeMessage{}, nil
	case INTERESTED_MESSAGE:
		slog.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"Received interested",
			slog.String("peerIP", pc.otherPeer.ip),
			slog.Int("PeerPort", int(pc.otherPeer.port)),
		)
		return interestedMessage{}, nil
	case NOT_INTERESTED_MESSAGE:
		slog.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"Received not interested",
			slog.String("peerIP", pc.otherPeer.ip),
			slog.Int("PeerPort", int(pc.otherPeer.port)),
		)
		return notInterestedMessage{}, nil
	case HAVE_MESSAGE:
		if payloadLen != 4 {
			return nil, &PeerCommunicationError{Message: "Received 'have' message doesn't match expected structure"}
		}

		pieceIndex := binary.BigEndian.Uint32(msgBuf[:payloadLen])
		slog.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"Received have",
			slog.Int("PieceIndex", int(pieceIndex)),
			slog.String("peerIP", pc.otherPeer.ip),
			slog.Int("PeerPort", int(pc.otherPeer.port)),
		)
		return haveMessage{pieceIndex: int(pieceIndex)}, nil
	case BITFIELD_MESSAGE:
		slog.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"Received bitfield",
			slog.String("peerIP", pc.otherPeer.ip),
			slog.Int("PeerPort", int(pc.otherPeer.port)),
		)
		return bitfieldMessage{bitfield: msgBuf[:payloadLen]}, nil
	case REQUEST_MESSAGE:
		if payloadLen != 12 {
			return nil, &PeerCommunicationError{Message: "Received 'request' message doesn't match expected structure"}
		}

		index := int(binary.BigEndian.Uint32(msgBuf[:4]))
		begin := int(binary.BigEndian.Uint32(msgBuf[4:8]))
		length := int(binary.BigEndian.Uint32(msgBuf[8:12]))

		slog.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"Received block request",
			slog.Int("PieceIndex", index),
			slog.Int("BlockOffset", begin),
			slog.Int("BlockLength", length),
			slog.String("peerIP", pc.otherPeer.ip),
			slog.Int("PeerPort", int(pc.otherPeer.port)),
		)
		return requestMessage{index: index, begin: begin, length: length}, nil
	case PIECE_MESSAGE:
		index := int(binary.BigEndian.Uint32(msgBuf[:4]))
		begin := int(binary.BigEndian.Uint32(msgBuf[4:8]))
		block := msgBuf[8:payloadLen]

		slog.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"Received piece block",
			slog.Int("PieceIndex", index),
			slog.Int("BlockOffset", begin),
			slog.Int("BlockLength", len(block)),
			slog.String("peerIP", pc.otherPeer.ip),
			slog.Int("PeerPort", int(pc.otherPeer.port)),
		)
		return pieceMessage{index: index, begin: begin, block: block}, nil
	case CANCEL_MESSAGE:
		if payloadLen != 12 {
			return nil, &PeerCommunicationError{Message: "Received 'cancel' message doesn't match expected structure"}
		}

		index := int(binary.BigEndian.Uint32(msgBuf[:4]))
		begin := int(binary.BigEndian.Uint32(msgBuf[4:8]))
		length := int(binary.BigEndian.Uint32(msgBuf[8:12]))

		slog.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"Received cancel request",
			slog.Int("PieceIndex", index),
			slog.Int("BlockOffset", begin),
			slog.Int("BlockLength", length),
			slog.String("peerIP", pc.otherPeer.ip),
			slog.Int("PeerPort", int(pc.otherPeer.port)),
		)
		return cancelMessage{index: index, begin: begin, length: length}, nil
	default:
		return nil, &PeerCommunicationError{Message: "Received message doesn't match any message supported by this app."}
	}
}

func (pc *peerConnection) performHandshake(infohash, peerID customdatatypes.CustomHash) error {
	slog.LogAttrs(
		context.Background(),
		slog.LevelInfo,
		"Attempting handshake",
		slog.String("peerIP", pc.otherPeer.ip),
		slog.Int("PeerPort", int(pc.otherPeer.port)),
	)
	pstr := "BitTorrent protocol"
	var handshakeBuffer bytes.Buffer

	responseBuffer := make([]byte, 68) // TODO: I wonder if the pstr can be anything other than what the 1.0 version says it is...
	reservedBytes := make([]byte, 8)
	handshakeBuffer.WriteByte(byte(19))
	handshakeBuffer.WriteString(pstr)
	handshakeBuffer.Write(reservedBytes)
	handshakeBuffer.Write(infohash.HashBytes)
	handshakeBuffer.Write(peerID.HashBytes)

	encodedHandshake := handshakeBuffer.Bytes()

	_, writeErr := pc.connection.Write(encodedHandshake)
	if writeErr != nil {
		return &PeerConnectionError{Message: "Failed to send handshake", InvolvedPeer: pc.otherPeer}
	}

	_, readErr := pc.connection.Read(responseBuffer)

	if readErr != nil {
		return &PeerConnectionError{Message: "Didn't receive handshake response", InvolvedPeer: pc.otherPeer}
	}
	// TODO: I really should be checking what the other client sent...

	slog.LogAttrs(
		context.Background(),
		slog.LevelInfo,
		"Handshake succeeded!",
		slog.String("peerIP", pc.otherPeer.ip),
		slog.Int("PeerPort", int(pc.otherPeer.port)),
	)
	return nil
}

func (pc *peerConnection) sendKeepAlive() error {
	// This message only has a length of 0. No ID, no payload
	keepAliveMsg := []byte{0x00, 0x00, 0x00, 0x00}
	_, writeErr := pc.connection.Write(keepAliveMsg)

	if writeErr != nil {
		return &PeerConnectionError{Message: "Failed to send keep-alive message.", InvolvedPeer: pc.otherPeer}
	}

	return nil
}

func (pc *peerConnection) sendChoke() error {
	// Message length is 1
	msgLength := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLength, uint32(1))

	// Message ID is 0
	msgID := byte(0)

	var chokeBuffer bytes.Buffer

	chokeBuffer.Write(msgLength)
	chokeBuffer.WriteByte(msgID)

	encodedChokeMsg := chokeBuffer.Bytes()

	_, writeErr := pc.connection.Write(encodedChokeMsg)
	if writeErr != nil {
		return &PeerCommunicationError{Message: "Couldn't send choke", InvolvedPeer: pc.otherPeer}
	}

	return nil
}

func (pc *peerConnection) sendUnchoke() error {
	// Message length is 1
	msgLength := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLength, uint32(1))

	// Message ID is 1
	msgID := byte(1)
	var unchokeBuffer bytes.Buffer

	unchokeBuffer.Write(msgLength)
	unchokeBuffer.WriteByte(msgID)

	encodedUnchokeMsg := unchokeBuffer.Bytes()

	_, writeErr := pc.connection.Write(encodedUnchokeMsg)
	if writeErr != nil {
		return &PeerCommunicationError{Message: "Couldn't send unchoke", InvolvedPeer: pc.otherPeer}
	}

	return nil
}

func (pc *peerConnection) sendInterested() error {
	// Message length is 1
	msgLength := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLength, uint32(1))

	// Message ID is 2
	msgID := byte(2)

	var interestedBuffer bytes.Buffer
	interestedBuffer.Write(msgLength)
	interestedBuffer.WriteByte(msgID)

	encodedInterestedMsg := interestedBuffer.Bytes()

	_, writeErr := pc.connection.Write(encodedInterestedMsg)
	if writeErr != nil {
		return &PeerCommunicationError{Message: "Couldn't send interested", InvolvedPeer: pc.otherPeer}
	}

	return nil
}

func (pc *peerConnection) sendNotInterested() error {
	// Message length is 1
	msgLength := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLength, uint32(1))

	// Message ID is 3
	msgID := byte(3)

	var notInterestedBuffer bytes.Buffer

	notInterestedBuffer.Write(msgLength)
	notInterestedBuffer.WriteByte(msgID)

	encodedNotInterestedMsg := notInterestedBuffer.Bytes()

	_, writeErr := pc.connection.Write(encodedNotInterestedMsg)
	if writeErr != nil {
		return &PeerCommunicationError{Message: "Couldn't send not interested", InvolvedPeer: pc.otherPeer}
	}

	return nil
}

func (pc *peerConnection) sendHave(pieceIndex int) error {
	// Message length is 1
	msgLength := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLength, uint32(5))

	// Message ID is 4
	msgID := byte(4)

	pieceIndexBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(pieceIndexBuf, uint32(pieceIndex))

	var haveBuffer bytes.Buffer

	haveBuffer.Write(msgLength)
	haveBuffer.WriteByte(msgID)
	haveBuffer.Write(pieceIndexBuf)

	encodedHaveMsg := haveBuffer.Bytes()

	_, writeErr := pc.connection.Write(encodedHaveMsg)
	if writeErr != nil {
		return &PeerCommunicationError{Message: "Couldn't send have", InvolvedPeer: pc.otherPeer}
	}

	return nil
}

func (pc *peerConnection) sendRequest(index, begin, length int) error {
	// Message length is 13
	msgLength := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLength, uint32(13))

	// Message ID is 6
	msgID := byte(6)

	// index represents the index of the piece
	indexBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBuf, uint32(index))

	// begin represents the offset of the block we are requesting
	beginBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(beginBuf, uint32(begin))

	// length represents the size of the block we are requesting.
	// There is debate around the appropriate size, but 16 KB seems to be a good number
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(length))

	var requestBuffer bytes.Buffer

	requestBuffer.Write(msgLength)
	requestBuffer.WriteByte(msgID)
	requestBuffer.Write(indexBuf)
	requestBuffer.Write(beginBuf)
	requestBuffer.Write(lengthBuf)

	encodedRequestMsg := requestBuffer.Bytes()

	_, writeErr := pc.connection.Write(encodedRequestMsg)
	if writeErr != nil {
		return &PeerCommunicationError{Message: "Couldn't send request", InvolvedPeer: pc.otherPeer}
	}

	return nil
}

func (pc *peerConnection) sendPiece(index, begin int, block []byte) error {
	// Message length is 9 + the size of the block
	msgLength := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLength, uint32(9)+uint32(len(block)))

	// Message ID is 7
	msgID := byte(7)

	// index represents the index of the piece
	indexBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBuf, uint32(index))

	// begin represents the offset of the block we are requesting
	beginBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(beginBuf, uint32(begin))

	var pieceBuffer bytes.Buffer

	pieceBuffer.Write(msgLength)
	pieceBuffer.WriteByte(msgID)
	pieceBuffer.Write(indexBuf)
	pieceBuffer.Write(beginBuf)
	pieceBuffer.Write(block)

	encodedPieceMsg := pieceBuffer.Bytes()

	_, writeErr := pc.connection.Write(encodedPieceMsg)
	if writeErr != nil {
		return &PeerCommunicationError{Message: "Couldn't send piece message.", InvolvedPeer: pc.otherPeer}
	}

	return nil
}

func (pc *peerConnection) sendCancel(index, begin, length int) error {
	// Message length is 13
	msgLength := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLength, uint32(13))

	// Message ID is 8
	msgID := byte(8)

	// index represents the index of the piece
	indexBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBuf, uint32(index))

	// begin represents the offset of the block we are cancelling
	beginBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(beginBuf, uint32(begin))

	// length represents the size of the block we are cancelling
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(length))

	var cancelBuffer bytes.Buffer

	cancelBuffer.Write(msgLength)
	cancelBuffer.WriteByte(msgID)
	cancelBuffer.Write(indexBuf)
	cancelBuffer.Write(beginBuf)
	cancelBuffer.Write(lengthBuf)

	encodedCancelMsg := cancelBuffer.Bytes()

	_, writeErr := pc.connection.Write(encodedCancelMsg)
	if writeErr != nil {
		return &PeerCommunicationError{Message: "Couldn't send piece cancellation message.", InvolvedPeer: pc.otherPeer}
	}

	return nil
}

func (pc *peerConnection) sendBitfield(clientPieceIndex customdatatypes.FixedSizeBitfield) error {
	bitfieldBytes := clientPieceIndex.ExposeBitfield()

	var bitfieldBuffer bytes.Buffer

	msgLength := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLength, uint32(len(bitfieldBytes)+1))
	msgID := make([]byte, 4)
	binary.BigEndian.PutUint32(msgID, uint32(5))

	bitfieldBuffer.Write(msgLength)
	bitfieldBuffer.Write(msgID)
	bitfieldBuffer.Write(bitfieldBytes)

	encodedBitfieldMsg := bitfieldBuffer.Bytes()

	_, writeErr := pc.connection.Write(encodedBitfieldMsg)
	if writeErr != nil {
		return &PeerCommunicationError{Message: "Couldn't send bitfield", InvolvedPeer: pc.otherPeer}
	}

	return nil
}
