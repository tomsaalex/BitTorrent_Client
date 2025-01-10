package Bytesources

import (
	"bufio"
	"io"

	"github.com/tomsaalex/BitTorrent_Client/backend/bencoding/bparserrs"
)

type FileSource struct {
	BufferedReader *bufio.Reader
}

func (fs *FileSource) Peek() (byte, error) {
	peekedByte, err := fs.BufferedReader.Peek(1)

	if err != nil {
		return 0, &bparserrs.DecodingError{Message: "Cannot Peek past source's end"}
	}

	return peekedByte[0], nil
}

func (fs *FileSource) Read() (byte, error) {
	readByte, err := fs.BufferedReader.ReadByte()

	if err != nil {
		return 0, &bparserrs.DecodingError{Message: "Cannot Read past source's end"}
	}

	return readByte, nil
}

func (fs *FileSource) ReadMultiple(byteCount int) ([]byte, error) {
	readingBuffer := make([]byte, byteCount)
	_, err := io.ReadFull(fs.BufferedReader, readingBuffer)

	if err != nil {
		return nil, &bparserrs.DecodingError{Message: "Cannot ReadMultiple past source's end"}
	}

	return readingBuffer, nil
}
