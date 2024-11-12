package ByteSources

import "github.com/tomsaalex/BitTorrent_Client/Bencoder/ParsingErrors"

type StringSource struct {
	StringData string
	Position   int
}

func (ss *StringSource) Peek() (byte, error) {
	if ss.Position >= len(ss.StringData) {
		return 0, &ParsingErrors.DecodingError{Message: "Cannot Peek past source's end"}
	}
	return ss.StringData[ss.Position], nil
}

func (ss *StringSource) Read() (byte, error) {
	if ss.Position >= len(ss.StringData) {
		return 0, &ParsingErrors.DecodingError{Message: "Cannot Read past source's end"}
	}
	readValue := ss.StringData[ss.Position]
	ss.Position++
	return readValue, nil
}

func (ss *StringSource) ReadMultiple(byteCount int) ([]byte, error) {
	if ss.Position+byteCount-1 >= len(ss.StringData) {
		return nil, &ParsingErrors.DecodingError{Message: "Cannot ReadMultiple past source's end"}
	}

	readString := ss.StringData[ss.Position : ss.Position+byteCount]
	ss.Position += byteCount
	return []byte(readString), nil
}
