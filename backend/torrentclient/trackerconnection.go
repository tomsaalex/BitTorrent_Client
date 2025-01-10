package torrentclient

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	Bencoding "github.com/tomsaalex/BitTorrent_Client/backend/bencoding"
	"github.com/tomsaalex/BitTorrent_Client/backend/bencoding/bparserrs"
	"github.com/tomsaalex/BitTorrent_Client/backend/customdatatypes"
)

type trackerConnection struct {
	c Bencoding.Codec
}

type trackerResponse struct {
	failureReason  string
	warningMessage string
	interval       int
	minInterval    int
	trackerID      string
	complete       int
	incomplete     int
	peers          []peer
}

func (tc *trackerConnection) parseTrackerResponse(response string) (trackerResponse, error) {
	// Take the string response and Bdecode it
	bencodedResponse, err := tc.c.DecodeString(response)

	if err != nil {
		return trackerResponse{}, &TrackerConnectionError{Message: "Couldn't parse tracker response: " + fmt.Sprintf("%s", err.Error())}
	}

	// Check that Bdecoded response is a dictionary, as demanded by protocol

	mainDictionaryValue, conversionSuccessful := bencodedResponse.(Bencoding.BencodableMap)

	if !conversionSuccessful {
		return trackerResponse{}, &bparserrs.DecodingError{Message: "Tracker response isn't a dictionary."}
	}

	// Identify present values in the decoded dictionary and extract them one by one

	var processedResponse trackerResponse

	failureReason, valuePresent := mainDictionaryValue["failure reason"].(Bencoding.BencodableString)

	if valuePresent {
		processedResponse.failureReason = failureReason
		return processedResponse, nil
	}

	warningMessage, valuePresent := mainDictionaryValue["warning message"].(Bencoding.BencodableString)

	if valuePresent {
		processedResponse.warningMessage = warningMessage
	}

	interval, valuePresent := mainDictionaryValue["interval"].(Bencoding.BencodableInt)

	if !valuePresent {
		return trackerResponse{}, &TrackerConnectionError{Message: "Tracker didn't return the interval field (mandatory)."}
	}

	processedResponse.interval = interval

	minInterval, valuePresent := mainDictionaryValue["min interval"].(Bencoding.BencodableInt)

	if valuePresent {
		processedResponse.minInterval = minInterval
	}

	trackerID, valuePresent := mainDictionaryValue["tracker id"].(Bencoding.BencodableString)

	if valuePresent {
		processedResponse.trackerID = trackerID
	}

	complete, valuePresent := mainDictionaryValue["complete"].(Bencoding.BencodableInt)

	if valuePresent {
		processedResponse.complete = complete
	}

	incomplete, valuePresent := mainDictionaryValue["incomplete"].(Bencoding.BencodableInt)

	if valuePresent {
		processedResponse.incomplete = incomplete
	}

	// Handle peers list in binary model

	peers, valuePresent := mainDictionaryValue["peers"].(Bencoding.BencodableString)

	if !valuePresent {
		return trackerResponse{}, &TrackerConnectionError{Message: "Tracker didn't return the list of peers (mandatory)."}
	}

	var buf bytes.Buffer
	var processedPeers = make([]peer, 0)

	var peerIP string
	var peerPort uint16
	for _, character := range []byte(peers) {
		buf.WriteByte(character)

		if buf.Len() == 6 {
			// TODO This works but it doesn't look pretty. Maybe extract it into a separate function or rewrite it better.
			peerIP = ""
			peerIP += strconv.Itoa(int(buf.Next(1)[0])) + "."
			peerIP += strconv.Itoa(int(buf.Next(1)[0])) + "."
			peerIP += strconv.Itoa(int(buf.Next(1)[0])) + "."
			peerIP += strconv.Itoa(int(buf.Next(1)[0]))
			peerPort = binary.BigEndian.Uint16(buf.Next(2))
			processedPeers = append(processedPeers, peer{ip: peerIP, port: peerPort})
		}
	}

	if buf.Len() != 0 {
		return trackerResponse{}, &TrackerConnectionError{Message: "Tracker's peer list is malformed."}
	}

	processedResponse.peers = processedPeers

	return processedResponse, nil
}

func (tc *trackerConnection) announceRequest(td TorrentData, ts TorrentStats, peerID customdatatypes.CustomHash, te TrackerEvent) (trackerResponse, error) {
	requestURL := fmt.Sprintf("%s?", td.Announce)

	requestParameters := url.Values{}

	torrentSize := 0

	if len(td.Files) > 0 {
		for _, fileData := range td.Files {
			torrentSize += fileData.FileLength
		}
	} else {
		torrentSize = td.FileLength
	}

	requestParameters.Add("info_hash", string(td.Infohash.HashBytes))
	requestParameters.Add("peer_id", string(peerID.HashBytes))
	//requestParameters.Add("ip", "tomsa.go.ro")                                  // Replace this with something proper
	requestParameters.Add("port", "6881")                                       // Replace this with the proper port
	requestParameters.Add("uploaded", strconv.Itoa(ts.uploadedBytes))           //
	requestParameters.Add("downloaded", strconv.Itoa(ts.downloadedBytes))       //
	requestParameters.Add("left", strconv.Itoa(torrentSize-ts.downloadedBytes)) //
	requestParameters.Add("compact", "1")

	if te != T_NIL {
		var eventString string
		switch te {
		case T_STARTED:
			eventString = "started"
		case T_STOPPED:
			eventString = "stopped"
		case T_COMPLETED:
			eventString = "completed"
		}

		requestParameters.Add("event", eventString)
	}

	requestURL += requestParameters.Encode()

	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		errMessage := fmt.Sprintf("TrackerConnection: could not create announce request: %s\n", err)
		panic(TrackerConnectionError{Message: errMessage})
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		errMessage := fmt.Sprintf("TrackerConnection: error making announce request: %s\n", err)
		panic(TrackerConnectionError{Message: errMessage})
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		errMessage := fmt.Sprintf("TrackerConnection: could not read response body: %s\n", err)
		panic(TrackerConnectionError{Message: errMessage})
	}

	return tc.parseTrackerResponse(string(resBody))
}
