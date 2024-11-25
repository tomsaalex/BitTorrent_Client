package customdatatypes

import "encoding/hex"

type CustomHash struct {
	HashBytes []byte
}

func (ch *CustomHash) String() string {
	return hex.EncodeToString(ch.HashBytes)
}
