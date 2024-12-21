package torrentclient

import (
	"bytes"
	"context"
	"crypto/sha1"
	"log/slog"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/tomsaalex/BitTorrent_Client/customdatatypes"
)

const piecesDownloadNum int = 1
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

func (pcm *PeerConnectionManager) establishConnection(peer peer, tData TorrentData, connectionOutput chan connMessage, peerID customdatatypes.CustomHash) (peerConnection, error) {
	newConnection, err := newPeerConnection(peer, tData, connectionOutput)

	if err != nil {
		return peerConnection{}, err
	}

	err = newConnection.performHandshake(tData.Infohash, peerID)

	if err != nil {
		return peerConnection{}, err
	}

	return newConnection, nil
}

func (pcm *PeerConnectionManager) establishConnections(peersList []peer, tData TorrentData, peerID customdatatypes.CustomHash, connectionOutput chan connMessage) {
	for _, peer := range peersList {
		peerConnection, err := pcm.establishConnection(peer, tData, connectionOutput, peerID)

		if err != nil {
			slog.LogAttrs(
				context.Background(),
				slog.LevelError,
				err.Error(),
			) // TODO: Perhaps do better logging for this. Or handle it in a better place, if you think there is one.
			continue
		}

		pcm.peerConnections = append(pcm.peerConnections, peerConnection)

		go peerConnection.connectionRoutine()
	}
}

func (pcm *PeerConnectionManager) connectionManager(torrentData TorrentData, tStats *TorrentStats, peerID customdatatypes.CustomHash, peerRequestChan chan<- bool, peersChan <-chan []peer, newPieceAcquiredChan chan<- piece, bitfieldRequestChan chan<- bool, bitfieldOutputChan <-chan customdatatypes.FixedSizeBitfield) {
	peerRequestChan <- true

	connectionOutput := make(chan connMessage)

	requestedPieces := make([]BlockRequest, 0)

	// TODO: The problem might be here. The unchoke code never executes, even if we have a channel that triggers it right away.
	regularUnchokeTicker := time.NewTicker(10 * time.Second)

	/* // TODO: This is not in the specification.
	regularPieceScheduleTicker := time.NewTicker(5 * time.Second) */

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
			pcm.establishConnections(unconnectedPeers, torrentData, peerID, connectionOutput)
			if len(requestedPieces) == 0 {
				pcm.schedulePiecesForDownload(requestedPieces, tStats, piecesDownloadNum, torrentData.PieceLength)
			}
		case receivedMessage := <-connectionOutput:
			rawMessage := receivedMessage.peerMsg
			switch peerMessage := rawMessage.(type) {
			case pieceMessage:
				assemblerInput <- peerMessage
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
			newPieceAcquiredChan <- receivedPiece
			tStats.MarkPieceAsObtained(receivedPiece.pieceIndex)
			if len(requestedPieces) == 0 {
				pcm.schedulePiecesForDownload(requestedPieces, tStats, piecesDownloadNum, torrentData.PieceLength)
			}
		case <-regularUnchokeTicker.C:
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Peer change start",
				slog.String("method", "connectionManager"),
			)
			// TODO Replace peer after you implement optimistic unchoking
			pcm.changeUnchokedDownloaders(peer{}, requestedPieces)
			if len(requestedPieces) == 0 {
				pcm.schedulePiecesForDownload(requestedPieces, tStats, piecesDownloadNum, torrentData.PieceLength)
			}
			/* case <-regularPieceScheduleTicker.C:
			if len(requestedPieces) == 0 {
				pcm.schedulePiecesForDownload(requestedPieces, tStats, piecesDownloadNum, torrentData.PieceLength)
			} */
		}
	}
}

func (pcm *PeerConnectionManager) pieceAssembler(tData TorrentData, blockInput <-chan pieceMessage, pieceOutput chan<- piece) {
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
				pieceData := make([]byte, tData.PieceLength)
				pieceCatalogue[pieceIndex] = pieceData

				bitfield, err := customdatatypes.NewFixedSizeBitfield(tData.PieceLength)
				if err != nil {
					// TODO: Better error handling here, although the error is technically impossible
					panic(err)
				}

				changesCatalogue[pieceIndex] = bitfield
			}

			for i := 0; i < len(newBlock.block); i++ {
				pieceCatalogue[pieceIndex][i+blockOffset] = blockData[i]
				changesCatalogue[pieceIndex].SetBit(i)
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
}

func (pcm *PeerConnectionManager) changeUnchokedDownloaders(optimisticUnchokedP peer, blockRequests []BlockRequest) {
	for i := 0; i < len(pcm.peerConnections); i++ {
		pcm.peerConnections[i].speedRequests <- true
		pcm.peerConnections[i].dataTransferSpeedSnapshot = <-pcm.peerConnections[i].speedReplies
	}

	sort.Slice(pcm.peerConnections, func(i, j int) bool {
		return pcm.peerConnections[i].dataTransferSpeedSnapshot < pcm.peerConnections[j].dataTransferSpeedSnapshot
	})

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

	for _, conn := range pcm.peerConnections {
		if len(downloaders) >= 1 && conn.dataTransferSpeedSnapshot > downloaders[len(downloaders)-1].dataTransferSpeedSnapshot {
			uninterestedPeers = append(uninterestedPeers, conn)
		} else if len(downloaders) == 0 {
			// TODO: This if might be wrong. The protocol doesn't specify what to do if there are no downloaders, I don't think. Check later
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

func (pcm *PeerConnectionManager) schedulePiecesForDownload(requestedPieces []BlockRequest, tStats *TorrentStats, numPieces, pieceLength int) {
	// TODO Perhaps it's not okay to just ignore the peers that are choking us without sending an interested message
	// Check with protocol to decide how to improve this function.

	// TODO: This somehow starts running while the bitfield isn't loaded? It goes into an infinite loop still. Check
	for i := 0; i < numPieces; i++ {
		scheduleSuccessful := false

		if len(tStats.unselectedPieces) == 0 {
			return
		}

		for !scheduleSuccessful {
			randomIndex := rand.IntN(len(tStats.unselectedPieces))
			pieceIndex := tStats.unselectedPieces[randomIndex]

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
					newRequests := pcm.generateBlockRequests(conn, pieceIndex, pieceLength)

					for _, req := range newRequests {
						requestedPieces = append(requestedPieces, req)
						conn.input <- requestMessage{index: req.pieceIndex, begin: req.blockStart, length: req.blockLength}
					}

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

func (pcm *PeerConnectionManager) generateBlockRequests(peerConn peerConnection, pieceIndex, pieceLength int) []BlockRequest {
	begin := 0
	blockCount := pieceLength / blockSize

	newRequests := make([]BlockRequest, 0)

	for i := 0; i < blockCount; i++ {
		req := BlockRequest{pieceIndex: pieceIndex, blockStart: begin, blockLength: blockSize}
		newRequests = append(newRequests, req)
		begin += blockSize
	}

	return newRequests
}
