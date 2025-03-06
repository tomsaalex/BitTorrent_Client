package torrentclient

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sort"
	"strconv"
	"time"

	"github.com/tomsaalex/BitTorrent_Client/backend/customdatatypes"
)

const piecesDownloadNum int = 1
const downloadersNum int = 4
const unchokingInterval time.Duration = 10 * time.Second
const optimisticUnchokingInterval time.Duration = 30 * time.Second

const blockSize = 16 * 1024

type PeerConnectionManager struct {
	peerConnections []*peerConnection

	connectionIntegration chan connBootstrapInfo
}

func newPeerConnectionManager() *PeerConnectionManager {
	peerConnections := make([]*peerConnection, 0)
	connectionIntegration := make(chan connBootstrapInfo)
	return &PeerConnectionManager{peerConnections: peerConnections, connectionIntegration: connectionIntegration}
}

func (pcm *PeerConnectionManager) establishConnection(peer peer, tData TorrentData, connectionOutput chan connMessage, peerID customdatatypes.CustomHash, assemblerInput chan pieceMessage) (*peerConnection, error) {
	newConnection, err := newPeerConnection(peer, tData, connectionOutput, assemblerInput)

	if err != nil {
		return nil, err
	}

	err = newConnection.performHandshake(tData.Infohash, peerID)

	if err != nil {
		newConnection.connection.Close()
		return nil, err
	}

	return newConnection, nil
}

func (pcm *PeerConnectionManager) establishConnections(ctx context.Context, peersList []peer, tData TorrentData, tStats *TorrentStats, peerID customdatatypes.CustomHash, connectionOutput chan connMessage, assemblerInput chan pieceMessage) {
	for _, peer := range peersList {
		peerConnection, err := pcm.establishConnection(peer, tData, connectionOutput, peerID, assemblerInput)

		if err != nil {
			slog.LogAttrs(
				context.Background(),
				slog.LevelError,
				err.Error(),
			) // TODO: Perhaps do better logging for this. Or handle it in a better place, if you think there is one.
			continue
		}

		pcm.peerConnections = append(pcm.peerConnections, peerConnection)

		peerConnection.launchConnectionRoutines(ctx, tStats)
	}
}

func (pcm *PeerConnectionManager) establishIncomingConnection(ctx context.Context, connInfo connBootstrapInfo, tStats *TorrentStats, tData TorrentData, connectionOutput chan connMessage, peerID customdatatypes.CustomHash, assemblerInput chan pieceMessage) {
	peerConnection, err := setupIncomingConnection(connInfo, tData, connectionOutput, assemblerInput)
	if err != nil {
		// TODO: Perhaps do better logging for this. Or handle it in a better place, if you think there is one.
		return
	}

	err = peerConnection.sendHandshakeMsg(tData.Infohash, peerID)

	if err != nil {
		slog.LogAttrs(
			context.Background(),
			slog.LevelError,
			err.Error(),
		) // TODO: Perhaps do better logging for this. Or handle it in a better place, if you think there is one.
		return
	}

	peerConnection.otherPeer.peerID, err = peerConnection.receivePeerID()

	if err != nil {
		slog.LogAttrs(
			context.Background(),
			slog.LevelError,
			err.Error(),
		) // TODO: Perhaps do better logging for this. Or handle it in a better place, if you think there is one.
		return
	}

	pcm.peerConnections = append(pcm.peerConnections, peerConnection)

	peerConnection.launchConnectionRoutines(ctx, tStats)

	localBitfield := tStats.piecesStoredToDisk
	peerConnection.input <- bitfieldMessage{bitfield: localBitfield.ExposeBitfield()}
}

func (pcm *PeerConnectionManager) connectionManager(ctx context.Context, torrentData TorrentData, tStats *TorrentStats, peerID customdatatypes.CustomHash, peerRequestChan chan<- bool, peersChan <-chan []peer, pieceToWriter chan<- piece, havePieceAnnouncer <-chan int, blockRetrievalRequests chan<- BlockRetrievalRequest) {
	select {
	case peerRequestChan <- true:
		fmt.Println("Sent peer request")
	case <-ctx.Done():
		fmt.Println("Exitted out of connectionManager")
		return
	}

	connectionOutput := make(chan connMessage)

	regularUnchokeTicker := time.NewTicker(10 * time.Second)

	assemblerInput := make(chan pieceMessage)
	assemblerOutput := make(chan piece)

	go pcm.pieceAssembler(ctx, torrentData, tStats, assemblerInput, assemblerOutput)

	for {
		select {
		case newPeers := <-peersChan:
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Received list of peers from torrentManager",
				slog.String("method", "connectionManager"),
				slog.Int("peerCount", len(newPeers)),
			)
			if len(newPeers) == 0 {
				continue
			}

			oldConnNumber := len(pcm.peerConnections)

			unconnectedPeers := make([]peer, 0)
			for _, peer := range newPeers {
				connected := false
				for _, conn := range pcm.peerConnections {
					if conn.otherPeer.equal(&peer) {
						connected = true
						break
					}
				}
				if !connected {
					unconnectedPeers = append(unconnectedPeers, peer)
				}
			}
			pcm.establishConnections(ctx, unconnectedPeers, torrentData, tStats, peerID, connectionOutput, assemblerInput)

			if oldConnNumber == 0 {
				requestedPieces := tStats.requestedPiecesCopy()

				pcm.changeUnchokedDownloaders(ctx, peer{}, tStats)
				if len(requestedPieces) < piecesDownloadNum && len(tStats.unselectedPiecesCopy()) > 0 {
					pcm.schedulePiecesForDownload(ctx, &torrentData, tStats, piecesDownloadNum-len(tStats.requestedPieces), torrentData.PieceLength)
				}
			}
		case receivedMessage := <-connectionOutput:
			rawMessage := receivedMessage.peerMsg
			switch peerMessage := rawMessage.(type) {
			case pieceMessage:
				//fmt.Println("Got block. Piece index #" + strconv.Itoa(peerMessage.index) + " - block start: " + strconv.Itoa(peerMessage.begin))
				tStats.markBlockAsObtained(BlockRequest{pieceIndex: peerMessage.index, blockStart: peerMessage.begin})
				/*if len(requestedPieces) < piecesDownloadNum {
					pcm.schedulePiecesForDownload(&requestedPieces, &torrentData, tStats, piecesDownloadNum-len(requestedPieces), torrentData.PieceLength)
				}*/
			case unchokeMessage:
				// This makes the client start scheduling pieces faster, without waiting for the regular timer.
				// It prevents most of the annoying 10-15 seconds wait when the download starts.
				requestedPieces := tStats.requestedPiecesCopy()

				if len(requestedPieces) < piecesDownloadNum && len(tStats.unselectedPiecesCopy()) > 0 {
					pcm.schedulePiecesForDownload(ctx, &torrentData, tStats, piecesDownloadNum-len(requestedPieces), torrentData.PieceLength)
				}
			case requestMessage:
				validErr := validateRequestMessage(peerMessage, &torrentData)
				if validErr != nil {
					slog.LogAttrs(
						context.Background(),
						slog.LevelInfo,
						"Discarded invalid request",
						slog.String("method", "connectionManager"),
						slog.String("peerIP", receivedMessage.peerConn.otherPeer.ip),
						slog.Int("pieceIndex", peerMessage.index),
						slog.Int("blockStart", peerMessage.begin),
						slog.Int("blockLength", peerMessage.length),
					)
					continue
				}

				blockDestination := receivedMessage.peerConn.input
				select {
				case blockRetrievalRequests <- BlockRetrievalRequest{req: peerMessage, pieceOutput: blockDestination}:
				case <-ctx.Done():
					return
				}
			case cancelMessage:
				// TODO: Same as requestMessage, or when adding Endgame Mode, I suppose?
			case connectionDropMessage:
				pcm.dropConnection(receivedMessage.peerConn)
			}
		case receivedPiece := <-assemblerOutput:
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Piece completed",
				slog.String("method", "connectionManager"),
				slog.Int("pieceIndex", receivedPiece.pieceIndex),
			)

			select {
			case pieceToWriter <- receivedPiece:
			case <-ctx.Done():
				return
			}

			tStats.markPieceAsObtained(receivedPiece.pieceIndex)
			requestedPieces := tStats.requestedPiecesCopy()

			if len(requestedPieces) < piecesDownloadNum {
				pcm.schedulePiecesForDownload(ctx, &torrentData, tStats, piecesDownloadNum-len(requestedPieces), torrentData.PieceLength)
			}
		case pieceIndex := <-havePieceAnnouncer:
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Broadcasting 'have'",
				slog.Int("pieceIndex", pieceIndex),
			)
			for _, pc := range pcm.peerConnections {
				newMsg := haveMessage{pieceIndex: pieceIndex}
				select {
				case pc.input <- newMsg:
				case <-ctx.Done():
				}
			}
		case <-regularUnchokeTicker.C:
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Peer change start",
				slog.String("method", "connectionManager"),
			)
			requestedPieces := tStats.requestedPiecesCopy()

			// TODO: Replace peer{} after you implement optimistic unchoking
			pcm.changeUnchokedDownloaders(ctx, peer{}, tStats)
			if len(requestedPieces) < piecesDownloadNum && len(tStats.unselectedPiecesCopy()) > 0 {
				pcm.schedulePiecesForDownload(ctx, &torrentData, tStats, piecesDownloadNum-len(tStats.requestedPieces), torrentData.PieceLength)
			}
		case bootstrapInfo := <-pcm.connectionIntegration:
			pcm.establishIncomingConnection(ctx, bootstrapInfo, tStats, torrentData, connectionOutput, peerID, assemblerInput)
		case <-ctx.Done():
			pcm.dropAllConnections()
			fmt.Println("Exitted out of connectionManager")
			return
		}

	}
}

func (pcm *PeerConnectionManager) dropConnection(pc *peerConnection) {
	indexToDel := -1
	for i, conn := range pcm.peerConnections {
		if conn.otherPeer.equal(&pc.otherPeer) {
			indexToDel = i
			break
		}
	}

	if indexToDel > -1 {
		pcm.peerConnections[indexToDel].connection.Close()
		pcm.peerConnections = append(pcm.peerConnections[:indexToDel], pcm.peerConnections[indexToDel+1:]...)
	}
}

func (pcm *PeerConnectionManager) dropAllConnections() {
	for _, conn := range pcm.peerConnections {
		conn.connection.Close()
	}

	pcm.peerConnections = nil
}

func (pcm *PeerConnectionManager) pieceAssembler(ctx context.Context, tData TorrentData, tStats *TorrentStats, blockInput <-chan pieceMessage, pieceOutput chan<- piece) {
	pieceCatalogue := make(map[int][]byte)
	changesCatalogue := make(map[int]*customdatatypes.FixedSizeBitfield)
	for {
		select {
		case newBlock := <-blockInput:
			pieceIndex := newBlock.index
			blockOffset := newBlock.begin
			blockData := newBlock.block

			_, exists := pieceCatalogue[pieceIndex]

			if !exists {
				var pieceLength int
				var torrentLength int

				if len(tData.Files) > 0 {
					// Multiple files mode
					torrentLength = tData.TorrentSize
				} else {
					// Single file mode
					torrentLength = tData.FileLength
				}

				if pieceIndex == len(tData.PieceHashes)-1 && torrentLength%tData.PieceLength != 0 {
					pieceLength = torrentLength % tData.PieceLength
				} else {
					pieceLength = tData.PieceLength
				}

				pieceData := make([]byte, pieceLength)
				pieceCatalogue[pieceIndex] = pieceData
				bitfield, err := customdatatypes.NewFixedSizeBitfield(pieceLength)
				if err != nil {
					// TODO: Better error handling here, although the error is technically impossible
					panic(err)
				}

				changesCatalogue[pieceIndex] = bitfield
			}

			for i := 0; i < len(newBlock.block); i++ {
				pieceCatalogue[pieceIndex][i+blockOffset] = blockData[i]
				changesCatalogue[pieceIndex].SetBit(i + blockOffset)
			}

			if changesCatalogue[pieceIndex].IsFull() {
				var sha = sha1.New()
				sha.Write(pieceCatalogue[pieceIndex])
				pieceHash := sha.Sum(nil)[:20]

				hashCorrect := bytes.Equal(pieceHash, tData.PieceHashes[pieceIndex].HashBytes)

				if !hashCorrect {
					// TODO: Better error handling
					panic("Piece hash didn't match expected hash... Couldn't handle error.")
				}

				completePiece := piece{pieceIndex: pieceIndex, data: pieceCatalogue[pieceIndex]}
				delete(pieceCatalogue, pieceIndex)
				delete(changesCatalogue, pieceIndex)

				select {
				case pieceOutput <- completePiece:
					tStats.addPartialPiecesBytes(-len(completePiece.data))
				case <-ctx.Done():
					return
				}
			} else {
				tStats.addPartialPiecesBytes(len(newBlock.block))
			}
		case <-ctx.Done():
			fmt.Println("Exitted out of pieceAssembler")
			return
		}
	}
}

func sortConnectionsBySpeed(connections []*peerConnection, speeds []int) ([]*peerConnection, []int) {
	indices := make([]int, len(connections))

	for i := 0; i < len(indices); i++ {
		indices[i] = i
	}

	sort.Slice(indices, func(i, j int) bool {
		return speeds[indices[i]] < speeds[indices[j]]
	})

	sortedConnections := make([]*peerConnection, len(connections))
	sortedSpeeds := make([]int, len(connections))

	for i := 0; i < len(indices); i++ {
		sortedConnections[i] = connections[indices[i]]
		sortedSpeeds[i] = speeds[indices[i]]
	}

	return sortedConnections, sortedSpeeds
}

func (pcm *PeerConnectionManager) changeUnchokedDownloaders(ctx context.Context, optimisticUnchokedP peer, tStats *TorrentStats) {
	connSpeeds := make([]int, len(pcm.peerConnections))
	fmt.Println("Conn speeds has " + strconv.Itoa(len(connSpeeds)) + " positions")
	for i := 0; i < len(pcm.peerConnections); i++ {
		connSpeeds[i] = pcm.peerConnections[i].connDownloadSpeed()
	}

	pcm.peerConnections, connSpeeds = sortConnectionsBySpeed(pcm.peerConnections, connSpeeds)

	downloaders := make([]*peerConnection, 0)
	lastDownloaderSpeed := 0
	for i, conn := range pcm.peerConnections {
		connData := conn.getConnectionData()

		if connData.peerInterested {
			downloaders = append(downloaders, conn)
			if len(downloaders) == 4 {
				lastDownloaderSpeed = connSpeeds[i]
				break
			}
		}
	}

	uninterestedPeers := make([]*peerConnection, 0)

	for i, conn := range pcm.peerConnections {
		// TODO: This adds every peer if there are no downloaders. The protocol isn't too specific on whether this is what we need.
		if connSpeeds[i] >= lastDownloaderSpeed {
			uninterestedPeers = append(uninterestedPeers, conn)
		}
	}

	for _, generalConn := range pcm.peerConnections {
		mustStayUnchoked := false

		if generalConn.otherPeer.equal(&optimisticUnchokedP) {
			mustStayUnchoked = true
		}

		var generalConnData connData
		if len(downloaders) > 0 || len(uninterestedPeers) > 0 {
			generalConnData = generalConn.getConnectionData()
		}

		for i := 0; i < len(downloaders) && !mustStayUnchoked; i++ {
			if generalConn.otherPeer.equal(&downloaders[i].otherPeer) {
				mustStayUnchoked = true

				if generalConnData.amChoking {
					select {
					case generalConn.input <- unchokeMessage{}:
					case <-ctx.Done():
						return
					}
				}
			}
		}

		for i := 0; i < len(uninterestedPeers) && !mustStayUnchoked; i++ {
			if generalConn.otherPeer.equal(&uninterestedPeers[i].otherPeer) {
				mustStayUnchoked = true
				if generalConnData.amChoking {
					select {
					case generalConn.input <- unchokeMessage{}:
					case <-ctx.Done():
						return
					}
				}
			}
		}

		if !mustStayUnchoked {
			//pcm.cancelRequestsToConnection(blockRequests, generalConn)
			tStats.cancelRequestsToPeer(generalConn)

			select {
			case generalConn.input <- chokeMessage{}:
			case <-ctx.Done():
			}

		}
	}
}

func (pcm *PeerConnectionManager) schedulePiecesForDownload(ctx context.Context, tData *TorrentData, tStats *TorrentStats, numPieces, pieceLength int) {
	fmt.Println("Started scheduling pieces")
	for i := 0; i < numPieces; i++ {
		scheduleSuccessful := false

		unselectedPieces := tStats.unselectedPiecesCopy()

		if len(unselectedPieces) == 0 {
			fmt.Println("No more pieces left to request")
			return
		}

		for !scheduleSuccessful {
			randomIndex := rand.IntN(len(unselectedPieces))
			pieceIndex := unselectedPieces[randomIndex]

			anyConnAvailable := false
			for _, conn := range pcm.peerConnections {
				connData := conn.getConnectionData()
				pieceAvailable, _ := conn.peerHasPiece(pieceIndex)

				if pieceAvailable {
					if connData.amChoking {
						fmt.Println("Stopped because I'm choking")
						continue
					}
					if connData.peerChoking {
						select {
						case conn.input <- interestedMessage{}:
						case <-ctx.Done():
							return
						}

						fmt.Println("Stopped because I expressed interest")
						continue
					}
					anyConnAvailable = true
					newRequests := pcm.generateBlockRequests(tData, pieceIndex, pieceLength)

					for _, req := range newRequests {
						tStats.markBlockAsRequested(req)
						reqMsg := requestMessage{index: req.pieceIndex, begin: req.blockStart, length: req.blockLength}
						select {
						case conn.input <- reqMsg:
						case <-ctx.Done():
							return
						}

						//fmt.Println("Made request for " + strconv.Itoa(req.pieceIndex) + " - " + strconv.Itoa(req.blockStart))
					}

					tStats.markPieceAsRequested(pieceIndex)

					scheduleSuccessful = true
					break
				}
			}

			if !anyConnAvailable {
				fmt.Println("Nobody to request from")
				return
			}
		}
	}
}

func (pcm *PeerConnectionManager) generateBlockRequests(tData *TorrentData, pieceIndex, pieceLength int) []BlockRequest {
	begin := 0
	isLastPiece := len(tData.PieceHashes)-1 == pieceIndex

	if isLastPiece {
		var lastPieceLength int
		if len(tData.Files) > 0 {
			lastPieceLength = tData.TorrentSize % pieceLength
		} else {
			lastPieceLength = tData.FileLength % pieceLength
		}

		if lastPieceLength != 0 {
			pieceLength = lastPieceLength
		}
	}

	blockCount := pieceLength / blockSize
	remainderData := pieceLength % blockSize
	if remainderData != 0 {
		blockCount++
	}

	newRequests := make([]BlockRequest, 0)

	for i := 0; i < blockCount; i++ {
		var req BlockRequest
		if remainderData != 0 && i == blockCount-1 {
			req = BlockRequest{pieceIndex: pieceIndex, blockStart: begin, blockLength: remainderData}
		} else {
			req = BlockRequest{pieceIndex: pieceIndex, blockStart: begin, blockLength: blockSize}
		}
		newRequests = append(newRequests, req)
		begin += blockSize
	}

	return newRequests
}
