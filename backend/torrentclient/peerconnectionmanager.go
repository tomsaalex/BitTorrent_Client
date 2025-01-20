package torrentclient

import (
	"bytes"
	"context"
	"crypto/sha1"
	"log/slog"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/tomsaalex/BitTorrent_Client/backend/customdatatypes"
)

const piecesDownloadNum int = 20
const downloadersNum int = 4
const unchokingInterval time.Duration = 10 * time.Second
const optimisticUnchokingInterval time.Duration = 30 * time.Second

const blockSize = 16 * 1024

type PeerConnectionManager struct {
	peerConnections []peerConnection
}

func newPeerConnectionManager() *PeerConnectionManager {
	peerConnections := make([]peerConnection, 0)
	return &PeerConnectionManager{peerConnections: peerConnections}
}

func (pcm *PeerConnectionManager) establishConnection(peer peer, tData TorrentData, connectionOutput chan connMessage, peerID customdatatypes.CustomHash, assemblerInput chan pieceMessage) (peerConnection, error) {
	newConnection, err := newPeerConnection(peer, tData, connectionOutput, assemblerInput)

	if err != nil {
		return peerConnection{}, err
	}

	err = newConnection.performHandshake(tData.Infohash, peerID)

	if err != nil {
		return peerConnection{}, err
	}

	return newConnection, nil
}

func (pcm *PeerConnectionManager) establishConnections(peersList []peer, tData TorrentData, peerID customdatatypes.CustomHash, connectionOutput chan connMessage, assemblerInput chan pieceMessage, dataReportsChan chan dataExchangeReport, speedReportChan chan speedExchangeReport, peerPieceChan chan peerPieceReport) {
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

		peerConnection.launchConnectionRoutines(dataReportsChan, speedReportChan, peerPieceChan)
	}
}

func (pcm *PeerConnectionManager) connectionManager(torrentData TorrentData, tStats *TorrentStats, peerID customdatatypes.CustomHash, peerRequestChan chan<- bool, peersChan <-chan []peer, pieceToWriter chan<- piece, havePieceAnnouncer <-chan int) {
	peerRequestChan <- true

	connectionOutput := make(chan connMessage)

	requestedPieces := make([]BlockRequest, 0)

	regularUnchokeTicker := time.NewTicker(10 * time.Second)

	assemblerInput := make(chan pieceMessage)
	assemblerOutput := make(chan piece)

	go pcm.pieceAssembler(torrentData, assemblerInput, assemblerOutput)

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
			pcm.establishConnections(unconnectedPeers, torrentData, peerID, connectionOutput, assemblerInput, tStats.dataReportsChan, tStats.speedReportsChan, tStats.newPeerPiecesChan)
			if len(requestedPieces) == 0 {
				pcm.schedulePiecesForDownload(&requestedPieces, &torrentData, tStats, piecesDownloadNum, torrentData.PieceLength)
			}
		case receivedMessage := <-connectionOutput:
			rawMessage := receivedMessage.peerMsg
			switch peerMessage := rawMessage.(type) {
			case pieceMessage:
				// Remove the piece from the list of requested blocks.
				for i := len(requestedPieces) - 1; i >= 0; i-- {
					if requestedPieces[i].pieceIndex == peerMessage.index && requestedPieces[i].blockStart == peerMessage.begin {
						requestedPieces = append(requestedPieces[:i], requestedPieces[i+1:]...)
					}
				}

				if len(requestedPieces) != piecesDownloadNum {
					pcm.schedulePiecesForDownload(&requestedPieces, &torrentData, tStats, piecesDownloadNum-len(requestedPieces), torrentData.PieceLength)
				}
			case requestMessage:
				// TODO: Add this when you implement uploading content
			case cancelMessage:
				// TODO Same as requestMessage, or when adding Endgame Mode, I suppose?
			}
		case receivedPiece := <-assemblerOutput:
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Piece completed",
				slog.String("method", "connectionManager"),
				slog.Int("pieceIndex", receivedPiece.pieceIndex),
			)

			pieceToWriter <- receivedPiece

			tStats.obtainedPiecesChan <- receivedPiece.pieceIndex

			if len(requestedPieces) == 0 {
				pcm.schedulePiecesForDownload(&requestedPieces, &torrentData, tStats, piecesDownloadNum, torrentData.PieceLength)
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
				pc.input <- newMsg
			}

		case <-regularUnchokeTicker.C:
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Peer change start",
				slog.String("method", "connectionManager"),
			)
			// TODO Replace peer{} after you implement optimistic unchoking
			pcm.changeUnchokedDownloaders(peer{}, requestedPieces)
			if len(requestedPieces) == 0 {
				pcm.schedulePiecesForDownload(&requestedPieces, &torrentData, tStats, piecesDownloadNum, torrentData.PieceLength)
			}
		}
	}
}

func (pcm *PeerConnectionManager) pieceAssembler(tData TorrentData, blockInput <-chan pieceMessage, pieceOutput chan<- piece) {
	pieceCatalogue := make(map[int][]byte)
	changesCatalogue := make(map[int]*customdatatypes.FixedSizeBitfield)

	for newBlock := range blockInput {
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

			if pieceIndex == len(tData.PieceHashes)-1 {
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

			pieceOutput <- completePiece
		}
	}
}

func sortConnectionsBySpeed(connections []peerConnection, speeds []int) ([]peerConnection, []int) {
	indices := make([]int, len(connections))

	for i := 0; i < len(indices); i++ {
		indices[i] = i
	}

	sort.Slice(indices, func(i, j int) bool {
		return speeds[indices[i]] < speeds[indices[j]]
	})

	sortedConnections := make([]peerConnection, len(connections))
	sortedSpeeds := make([]int, len(connections))

	for i := 0; i < len(indices); i++ {
		sortedConnections[i] = connections[indices[i]]
		sortedSpeeds[i] = speeds[indices[i]]
	}

	return sortedConnections, sortedSpeeds
}

func (pcm *PeerConnectionManager) changeUnchokedDownloaders(optimisticUnchokedP peer, blockRequests []BlockRequest) {
	connSpeeds := make([]int, len(pcm.peerConnections))
	for i := 0; i < len(pcm.peerConnections); i++ {
		pcm.peerConnections[i].speedRequests <- true
		connSpeeds[i] = <-pcm.peerConnections[i].speedReplies
	}

	pcm.peerConnections, connSpeeds = sortConnectionsBySpeed(pcm.peerConnections, connSpeeds)

	downloaders := make([]peerConnection, 0)
	for _, conn := range pcm.peerConnections {
		conn.connDataRequests <- true
		connData := <-conn.connDataReplies

		if connData.peerInterested {
			downloaders = append(downloaders, conn)
			if len(downloaders) == 4 {
				break
			}
		}
	}

	uninterestedPeers := make([]peerConnection, 0)

	lastDownloaderSpeed := 0
	if len(downloaders) > 0 {
		downloaders[len(downloaders)-1].speedRequests <- true
		lastDownloaderSpeed = <-downloaders[len(downloaders)-1].speedReplies
	}

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

		for i := 0; i < len(downloaders) && !mustStayUnchoked; i++ {
			if generalConn.otherPeer.equal(&downloaders[i].otherPeer) {
				mustStayUnchoked = true
				generalConn.connDataRequests <- true
				connData := <-generalConn.connDataReplies
				if connData.amChoking {
					generalConn.input <- unchokeMessage{}
				}
			}
		}

		for i := 0; i < len(uninterestedPeers) && !mustStayUnchoked; i++ {
			if generalConn.otherPeer.equal(&uninterestedPeers[i].otherPeer) {
				mustStayUnchoked = true
				generalConn.connDataRequests <- true
				connData := <-generalConn.connDataReplies
				if connData.amChoking {
					generalConn.input <- unchokeMessage{}
				}
			}
		}

		if !mustStayUnchoked {
			blockRequests = pcm.cancelRequestsToConnection(blockRequests, generalConn)
			generalConn.input <- chokeMessage{}
		}
	}
}

func (pcm *PeerConnectionManager) cancelRequestsToConnection(blockRequests []BlockRequest, pc peerConnection) []BlockRequest {
	validRequests := make([]BlockRequest, 0)
	for _, req := range blockRequests {
		if req.remotePeer.equal(&pc.otherPeer) {
			index := req.pieceIndex
			begin := req.blockStart
			length := req.blockLength
			pc.input <- cancelMessage{index: index, begin: begin, length: length}
		} else {
			validRequests = append(validRequests, req)
		}
	}

	return validRequests
}

func (pcm *PeerConnectionManager) schedulePiecesForDownload(requestedBlocks *[]BlockRequest, tData *TorrentData, tStats *TorrentStats, numPieces, pieceLength int) {
	for i := 0; i < numPieces; i++ {
		scheduleSuccessful := false

		tStats.unselectedPiecesRequest <- true
		unselectedPieces := <-tStats.unselectedPiecesReply

		if len(unselectedPieces) == 0 {
			return
		}

		for !scheduleSuccessful {
			randomIndex := rand.IntN(len(unselectedPieces))
			pieceIndex := unselectedPieces[randomIndex]

			// TODO: THIS ENTIRE MARKED SECTION IS NOT PROPERLY DONE
			// There are situations where you would want to re-request pieces, but that involves
			// proper tracking of requested pieces and a timeout for when to request them.
			// Redo soon
			pieceInProgress := false
			tStats.requestedPiecesRequest <- true
			requestedPieces := <-tStats.requestedPiecesReply
			for _, p := range requestedPieces {
				if p == pieceIndex {
					pieceInProgress = true
					break
				}
			}
			if pieceInProgress {
				break
			}
			for _, rp := range *requestedBlocks {
				if rp.pieceIndex == pieceIndex {
					pieceInProgress = true
					break
				}
			}
			if pieceInProgress {
				break
			}
			/////////////////////////////////////////////////////////
			anyConnAvailable := false
			for _, conn := range pcm.peerConnections {
				conn.connDataRequests <- true
				connData := <-conn.connDataReplies

				conn.pieceCheck <- pieceIndex
				pieceAvailable := <-conn.pieceStatus

				if pieceAvailable {
					if connData.amChoking {
						continue
					}
					if connData.peerChoking {
						conn.input <- interestedMessage{}
						continue
					}
					anyConnAvailable = true
					newRequests := pcm.generateBlockRequests(tData, pieceIndex, pieceLength)

					for _, req := range newRequests {
						*requestedBlocks = append(*requestedBlocks, req)
						conn.input <- requestMessage{index: req.pieceIndex, begin: req.blockStart, length: req.blockLength}
					}

					tStats.requestedPiecesChan <- pieceIndex

					scheduleSuccessful = true
					break
				}
			}

			if !anyConnAvailable {
				return
			}
		}
	}
}

func (pcm *PeerConnectionManager) generateBlockRequests(tData *TorrentData, pieceIndex, pieceLength int) []BlockRequest {
	begin := 0
	isLastPiece := len(tData.PieceHashes)-1 == pieceIndex

	if isLastPiece {
		if len(tData.Files) > 0 {
			pieceLength = tData.TorrentSize % pieceLength
		} else {
			pieceLength = tData.FileLength % pieceLength
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
