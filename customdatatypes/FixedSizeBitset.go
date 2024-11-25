package customdatatypes

import (
	"fmt"

	generalerrors "github.com/tomsaalex/BitTorrent_Client/GeneralErrors"
)

type FixedSizeBitfield struct {
	internalField []byte
	bitCount      int
}

func (bf *FixedSizeBitfield) SetBit(bitIndex int) error {
	if bitIndex >= bf.bitCount || bitIndex < 0 {
		return &generalerrors.IllegalAccessError{Message: fmt.Sprintf("Can't access given bit. Bit number %d is out of the bounds of the Bitfield with length %d", bitIndex, bf.bitCount)}
	}
	byteNum := bitIndex / 8
	bitToUpdate := bitIndex % 8

	bf.internalField[byteNum] |= (1 << (7 - bitToUpdate))

	return nil
}

func (bf *FixedSizeBitfield) ClearBit(bitIndex int) error {
	if bitIndex >= bf.bitCount || bitIndex < 0 {
		return &generalerrors.IllegalAccessError{Message: fmt.Sprintf("Can't access given bit. Bit number %d is out of the bounds of the Bitfield with length %d", bitIndex, bf.bitCount)}
	}
	byteNum := bitIndex / 8
	bitToUpdate := bitIndex % 8

	bf.internalField[byteNum] &^= (1 << (7 - bitToUpdate))

	return nil
}

func (bf *FixedSizeBitfield) IsSet(bitIndex int) (bool, error) {
	if bitIndex >= bf.bitCount || bitIndex < 0 {
		return false, &generalerrors.IllegalAccessError{Message: fmt.Sprintf("Can't access given bit. Bit number %d is out of the bounds of the Bitfield with length %d", bitIndex, bf.bitCount)}
	}

	byteNum := bitIndex / 8
	bitToQuery := bitIndex % 8

	return (bf.internalField[byteNum]>>(7-bitToQuery))&1 == 1, nil
}

func (bf *FixedSizeBitfield) ExposeBitfield() []byte {
	return bf.internalField
}

func NewFixedSizeBitfield(bitCount int) (*FixedSizeBitfield, error) {
	if bitCount <= 0 {
		return nil, &generalerrors.TypeInitializationError{TypeName: "FixedSizeBitfield", ErrorDetails: "Bitfield length cannot be 0 or negative."}
	}

	numBytes := bitCount / 8
	if bitCount%8 != 0 {
		numBytes++
	}
	return &FixedSizeBitfield{internalField: make([]byte, numBytes), bitCount: bitCount}, nil
}
