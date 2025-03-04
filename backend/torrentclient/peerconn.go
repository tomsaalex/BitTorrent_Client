package torrentclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/tomsaalex/BitTorrent_Client/backend/customdatatypes"
)

var failedSendCounter int = 0
var successSendCounter int = 0
var receiveCounter int = 0

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
	peerConn *peerConnection
	peerMsg  peerMessage
}

type TrafficType bool

const (
	IncomingTraffic TrafficType = false
	OutgoingTraffic             = true
)

type dataExchangeReport struct {
	remotePeer         peer
	exchangedDataCount int
	trafType           TrafficType
}

type speedExchangeReport struct {
	remotePeer peer
	connSpeed  int
	trafType   TrafficType
}

type peerPieceReport struct {
	remotePeer         peer
	reportedPieceIndex int
}

type peerConnection struct {
	otherPeer  peer
	connection net.Conn
	pieceCount int

	input          chan peerMessage
	output         chan connMessage
	assemblerInput chan pieceMessage

	downloadDataCounter   int
	downloadDataCounterMu sync.Mutex

	downloadTransferSpeed   int
	downloadTransferSpeedMu sync.Mutex

	downloadRates   []int
	downloadRatesMu sync.Mutex

	connectionData   connData
	connectionDataMu sync.Mutex

	peerBitfield *customdatatypes.FixedSizeBitfield
}

func newPeerConnection(peer peer, td TorrentData, output chan connMessage, assemblerInput chan pieceMessage) (*peerConnection, error) {
	connectAddress := peer.ip + ":" + strconv.Itoa(int(peer.port))

	conn, err := net.Dial("tcp", connectAddress)

	if err != nil {
		return nil, &PeerConnectionError{Message: "Failed to establish peer connection", InvolvedPeer: peer}
	}
	input := make(chan peerMessage)

	downloadDataCounter := 0
	downloadTransferSpeed := 0

	downloadRates := make([]int, 10)

	cd := connData{amChoking: true, amInterested: false, peerChoking: true, peerInterested: false}

	remotePeerBitfield, err := customdatatypes.NewFixedSizeBitfield(len(td.PieceHashes))
	if err != nil {
		return nil, &MalformedTorrentError{Message: "Couldn't initialize bitfield for torrent. Number of pieces invalid."}
	}

	peerConnection := peerConnection{
		otherPeer:             peer,
		connection:            conn,
		pieceCount:            len(td.PieceHashes),
		input:                 input,
		output:                output,
		assemblerInput:        assemblerInput,
		downloadDataCounter:   downloadDataCounter,
		downloadTransferSpeed: downloadTransferSpeed,
		downloadRates:         downloadRates,
		connectionData:        cd,
		peerBitfield:          remotePeerBitfield,
	}
	return &peerConnection, nil
}

func setupIncomingConnection(connInfo connBootstrapInfo, td TorrentData, output chan connMessage, assemblerInput chan pieceMessage) (*peerConnection, error) {
	input := make(chan peerMessage)

	tcpAddr := connInfo.conn.RemoteAddr().(*net.TCPAddr)

	otherPeer := peer{ip: tcpAddr.IP.String(), port: tcpAddr.AddrPort().Port()}

	downloadDataCounter := 0
	downloadTransferSpeed := 0

	downloadRates := make([]int, 10)

	cd := connData{amChoking: true, amInterested: false, peerChoking: true, peerInterested: false}

	remotePeerBitfield, err := customdatatypes.NewFixedSizeBitfield(len(td.PieceHashes))
	if err != nil {
		return nil, &MalformedTorrentError{Message: "Couldn't initialize bitfield for torrent. Number of pieces invalid."}
	}

	peerConnection := peerConnection{
		otherPeer:             otherPeer,
		connection:            connInfo.conn,
		pieceCount:            len(td.PieceHashes),
		input:                 input,
		output:                output,
		assemblerInput:        assemblerInput,
		downloadDataCounter:   downloadDataCounter,
		downloadTransferSpeed: downloadTransferSpeed,
		downloadRates:         downloadRates,
		connectionData:        cd,
		peerBitfield:          remotePeerBitfield,
	}

	return &peerConnection, nil
}

func calcAverageSpeed(dataPoints []int) int {
	average := 0
	for _, dp := range dataPoints {
		average += dp
	}

	// We're working with byte counts here, so we can afford to round without losing anything significant
	return average / len(dataPoints)
}

func (pc *peerConnection) launchConnectionRoutines(ctx context.Context, tStats *TorrentStats) {
	go pc.connDataManager(ctx, tStats)
	go pc.forwarderController(ctx)
	go pc.receiverController(ctx, tStats)
}

func (pc *peerConnection) forwarderController(ctx context.Context) {
	keepAliveTicker := time.NewTicker(KEEP_ALIVE_TIME)

	for {
		select {
		case rawMessage := <-pc.input:
			pc.forwarderRoutine(rawMessage)
			// TODO: Should really quantify the amount of data sent in the connDataManager routine
			switch rawMessage.(type) {
			case chokeMessage:
				pc.updateLocalConnState(Choked)
			case unchokeMessage:
				pc.updateLocalConnState(Unchoked)
			case interestedMessage:
				pc.updateLocalConnState(Interested)
			case notInterestedMessage:
				pc.updateLocalConnState(NotInterested)
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
		case <-ctx.Done():
			fmt.Println("Exitted out of forwarderController.")
			return
		}
	}
}

func (pc *peerConnection) receiverController(ctx context.Context, tStats *TorrentStats) {
	buffcon := bufio.NewReader(pc.connection)

	for {
		// TODO: Perhaps we shouldn't even be accepting data from choked peers? (we have to, but we should just dismiss some)
		rawMessage, err := pc.receiveMessage(buffcon)

		if err != nil {
			slog.LogAttrs(
				context.Background(),
				slog.LevelError,
				"Error receiving message from remote peer. Connection dropped.",
				slog.String("peerIP", pc.otherPeer.ip),
				slog.Int("PeerPort", int(pc.otherPeer.port)),
			)

			select {
			case pc.output <- connMessage{peerMsg: connectionDropMessage{}, peerConn: pc}:
			case <-ctx.Done():
			}
			return
		}

		connData := pc.getConnectionData()

		switch peerMessage := rawMessage.(type) {
		case chokeMessage:
			pc.updateRemoteConnState(Choked)
		case unchokeMessage:
			pc.updateRemoteConnState(Unchoked)
			pc.output <- connMessage{peerMsg: peerMessage, peerConn: pc}
		case interestedMessage:
			pc.updateRemoteConnState(Interested)
		case notInterestedMessage:
			pc.updateRemoteConnState(NotInterested)
		case haveMessage:
			pc.newHasFromPeer(tStats, peerMessage.pieceIndex)
		case pieceMessage:
			pc.assemblerInput <- peerMessage
			pc.output <- connMessage{peerMsg: peerMessage, peerConn: pc}
			pc.updateDownloadedAmount(tStats, len(peerMessage.block))
		case requestMessage:
			if connData.amChoking {
				break
			}
			pc.output <- connMessage{peerMsg: peerMessage, peerConn: pc}
		case cancelMessage:
			if connData.amChoking {
				break
			}
			pc.output <- connMessage{peerMsg: peerMessage, peerConn: pc}
		case bitfieldMessage:
			// TODO: Presumably, we should ignore it if it gets sent more than once?
			err := pc.setPeerBitfield(peerMessage.bitfield)
			if err != nil {
				select {
				case pc.output <- connMessage{peerMsg: connectionDropMessage{}, peerConn: pc}:
				case <-ctx.Done():
				}
				return
			}
		}
		/*case <-ctx.Done():
		fmt.Println("Exitted out of receiverController")
		return
		*/
	}
}

func (pc *peerConnection) updateLocalConnState(updateType StateUpdateType) {
	pc.connectionDataMu.Lock()
	defer pc.connectionDataMu.Unlock()

	switch updateType {
	case Choked:
		pc.connectionData.amChoking = true
	case Unchoked:
		pc.connectionData.amChoking = false
	case Interested:
		pc.connectionData.amInterested = true
	case NotInterested:
		pc.connectionData.amInterested = false
	}
}

func (pc *peerConnection) updateRemoteConnState(updateType StateUpdateType) {
	pc.connectionDataMu.Lock()
	defer pc.connectionDataMu.Unlock()

	switch updateType {
	case Choked:
		pc.connectionData.peerChoking = true
	case Unchoked:
		pc.connectionData.peerChoking = false
	case Interested:
		pc.connectionData.peerInterested = true
	case NotInterested:
		pc.connectionData.peerInterested = false
	}
}

func (pc *peerConnection) newHasFromPeer(tStats *TorrentStats, pieceIndex int) {
	pc.peerBitfield.SetBit(pieceIndex)
	tStats.updateRemotePeerPieces(peerPieceReport{remotePeer: pc.otherPeer, reportedPieceIndex: pieceIndex})
}

func (pc *peerConnection) updateDownloadedAmount(tStats *TorrentStats, downloadedAmount int) {
	pc.downloadDataCounterMu.Lock()
	defer pc.downloadDataCounterMu.Unlock()

	pc.downloadDataCounter += downloadedAmount
	tStats.addDataReport(dataExchangeReport{remotePeer: pc.otherPeer, trafType: IncomingTraffic, exchangedDataCount: downloadedAmount})
}

func (pc *peerConnection) getConnectionData() connData {
	pc.connectionDataMu.Lock()
	defer pc.connectionDataMu.Unlock()

	return pc.connectionData
}

func (pc *peerConnection) peerHasPiece(pieceIndex int) (bool, error) {
	return pc.peerBitfield.IsSet(pieceIndex)
}

func (pc *peerConnection) connDownloadSpeed() int {
	pc.downloadTransferSpeedMu.Lock()
	defer pc.downloadTransferSpeedMu.Unlock()

	return pc.downloadTransferSpeed
}

func (pc *peerConnection) setPeerBitfield(bitfield []byte) error {
	// TODO: Where this is called, if the error is not null, we must drop the connection
	return pc.peerBitfield.ImportBitfield(bitfield)
}

func (pc *peerConnection) updateDownloadSpeed(tStats *TorrentStats) {
	pc.downloadRatesMu.Lock()
	defer pc.downloadRatesMu.Unlock()

	pc.downloadDataCounterMu.Lock()
	defer pc.downloadDataCounterMu.Unlock()

	pc.downloadRates = append(pc.downloadRates[1:], pc.downloadDataCounter)

	pc.downloadTransferSpeedMu.Lock()
	defer pc.downloadTransferSpeedMu.Unlock()

	pc.downloadTransferSpeed = calcAverageSpeed(pc.downloadRates)

	pc.downloadDataCounter = 0
	tStats.addSpeedReport(speedExchangeReport{remotePeer: pc.otherPeer, connSpeed: pc.downloadTransferSpeed, trafType: IncomingTraffic})
}

func (pc *peerConnection) connDataManager(ctx context.Context, tStats *TorrentStats) {
	downloadRateTicker := time.NewTicker(CONNECTION_SPEED_UPDATE_TIME)

	for {
		select {
		case <-downloadRateTicker.C:
			pc.updateDownloadSpeed(tStats)
		case <-ctx.Done():
			return
		}
	}
}

/*func (pc *peerConnection) connDataManager(ctx context.Context, tStats *TorrentStats, forwarderStateUpdate <-chan StateUpdateType, receiverStateUpdate <-chan StateUpdateType, peerIndexUpdate <-chan int, receiverDataRateUpdate <-chan int, peerBitfieldUpdate <-chan []byte) {
	downloadDataCounter := 0
	downloadTransferSpeed := 0

	downloadRates := make([]int, 10)
	downloadRateTicker := time.NewTicker(CONNECTION_SPEED_UPDATE_TIME)

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
			// TODO: Now that we're storing all of the remote peer bitfields in the torrentStats, consider whether we should still store them here.
			// It might be a good idea for performance, aka leaving the one in torrentStats just for the sake of the UI.
			// Think on this later.
			tStats.updateRemotePeerPieces(peerPieceReport{remotePeer: pc.otherPeer, reportedPieceIndex: newHas})
		case dataReport := <-receiverDataRateUpdate:
			downloadDataCounter += dataReport
			tStats.addDataReport(dataExchangeReport{remotePeer: pc.otherPeer, trafType: IncomingTraffic, exchangedDataCount: dataReport})
		case <-pc.connDataRequests:
			pc.connDataReplies <- cd
		case index := <-pc.pieceCheck:
			pieceCheckResult, _ := peerBitfield.IsSet(index)
			pc.pieceStatus <- pieceCheckResult
		case <-pc.speedRequests:
			pc.speedReplies <- downloadTransferSpeed
		case <-downloadRateTicker.C:
			downloadRates = addDataPoint(downloadRates, downloadDataCounter)
			downloadTransferSpeed = calcConnectionSpeed(downloadRates)
			downloadDataCounter = 0
			tStats.addSpeedReport(speedExchangeReport{remotePeer: pc.otherPeer, connSpeed: downloadTransferSpeed, trafType: IncomingTraffic})
		case bitfield := <-peerBitfieldUpdate:
			// TODO: I think we need to check if any of the extra bits are set and drop the connection if so
			err := peerBitfield.ImportBitfield(bitfield)
			if err != nil {
				// TODO: Better error handling
				panic(err)
				// TODO: DROP CONNECTION
			}
		case <-ctx.Done():
			fmt.Println("Exitted out of connDataManager")
			return
		}
	}
}*/

func (pc *peerConnection) forwarderRoutine(message peerMessage) {
	switch peerMessage := message.(type) {
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
	case bitfieldMessage:
		bitfieldBytes := peerMessage.bitfield

		pc.sendBitfield(bitfieldBytes)

		slog.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"Sent bitfield",
			slog.String("peerIP", pc.otherPeer.ip),
			slog.Int("PeerPort", int(pc.otherPeer.port)),
		)
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

		receiveCounter++
		fmt.Println("Received blocks: " + strconv.Itoa(receiveCounter))
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

func (pc *peerConnection) sendHandshakeMsg(infohash, peerID customdatatypes.CustomHash) error {
	pstr := "BitTorrent protocol"
	var handshakeBuffer bytes.Buffer

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
	return nil
}

func (pc *peerConnection) receivePeerID() (customdatatypes.CustomHash, error) {
	// TODO: This could just be a generic function for receiving X bytes, maybe?
	peerIDBuff := make([]byte, 20)
	_, readErr := pc.connection.Read(peerIDBuff)
	if readErr != nil {
		return customdatatypes.CustomHash{}, &PeerConnectionError{Message: "Didn't receive peerID in incoming handshake", InvolvedPeer: pc.otherPeer}
	}

	peerID := customdatatypes.CustomHash{HashBytes: peerIDBuff}
	return peerID, nil
}

func (pc *peerConnection) performHandshake(infohash, peerID customdatatypes.CustomHash) error {
	slog.LogAttrs(
		context.Background(),
		slog.LevelInfo,
		"Attempting handshake",
		slog.String("peerIP", pc.otherPeer.ip),
		slog.Int("PeerPort", int(pc.otherPeer.port)),
	)
	responseBuffer := make([]byte, 68) // TODO: I wonder if the pstr can be anything other than what the 1.0 version says it is...

	writeErr := pc.sendHandshakeMsg(infohash, peerID)
	if writeErr != nil {
		return writeErr
	}
	// TODO: This is a hotfix because the client keeps trying to connect to itself. It's probably not a good way to handle timeouts.
	pc.connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, readErr := pc.connection.Read(responseBuffer)
	pc.connection.SetReadDeadline(time.Time{})

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
	// Message length is 5
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
		failedSendCounter++
		fmt.Println("Failed sent messages: " + strconv.Itoa(failedSendCounter) + " / Successful sent messages: " + strconv.Itoa(successSendCounter))

		return &PeerCommunicationError{Message: "Couldn't send request", InvolvedPeer: pc.otherPeer}

	}
	successSendCounter++
	fmt.Println("Failed sent messages: " + strconv.Itoa(failedSendCounter) + " / Successful sent messages: " + strconv.Itoa(successSendCounter))
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

func (pc *peerConnection) sendBitfield(bitfieldBytes []byte) error {
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
