package customdatatypes

import (
	"fmt"

	generalerrors "github.com/tomsaalex/BitTorrent_Client/backend/GeneralErrors"
)

type FixedSizeBitfield struct {
	internalField []byte
	bitCount      int
	bitsSet       int
}

func (bf *FixedSizeBitfield) BitCount() int {
	return bf.bitCount
}

func (bf *FixedSizeBitfield) SetBit(bitIndex int) error {
	if bitIndex >= bf.bitCount || bitIndex < 0 {
		return &generalerrors.IllegalAccessError{Message: fmt.Sprintf("Can't access given bit. Bit number %d is out of the bounds of the Bitfield with length %d", bitIndex, bf.bitCount)}
	}
	byteNum := bitIndex / 8
	bitToUpdate := bitIndex % 8

	oldByte := bf.internalField[byteNum]
	bf.internalField[byteNum] |= (1 << (7 - bitToUpdate))

	if oldByte != bf.internalField[byteNum] {
		bf.bitsSet++
	}

	return nil
}

func (bf *FixedSizeBitfield) ClearBit(bitIndex int) error {
	if bitIndex >= bf.bitCount || bitIndex < 0 {
		return &generalerrors.IllegalAccessError{Message: fmt.Sprintf("Can't access given bit. Bit number %d is out of the bounds of the Bitfield with length %d", bitIndex, bf.bitCount)}
	}
	byteNum := bitIndex / 8
	bitToUpdate := bitIndex % 8

	oldByte := bf.internalField[byteNum]
	bf.internalField[byteNum] &^= (1 << (7 - bitToUpdate))

	if oldByte != bf.internalField[byteNum] {
		bf.bitsSet--
	}

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

func (bf *FixedSizeBitfield) IsFull() bool {
	return bf.bitsSet == bf.bitCount
	/*
		////// KEEPING THIS HERE IN CASE THE NEW SOLUTION IS BROKEN IN SOME WAY //////
		numBytes := (bf.bitCount + 7) / 8

		for i := 0; i < numBytes-1; i++ {
			if bf.internalField[i] != 0xFF {
				return false
			}
		}

		lastByte := bf.internalField[numBytes-1]
		extraBitCount := (numBytes * 8) - bf.bitCount

		for i := 0; i < (8 - extraBitCount); i++ {
			if (lastByte>>(7-i))&1 == 0 {
				return false
			}
		}

		return true*/
}

func (bf *FixedSizeBitfield) ExposeBitfield() []byte {
	// Makes a copy, so you can't accidentally affect the bitfield without safe-guards.
	return append([]byte{}, bf.internalField...)
}

func (bf *FixedSizeBitfield) ImportBitfield(data []byte) error {
	expectedByteCount := (bf.bitCount + 7) / 8
	if len(data) != expectedByteCount {
		return &generalerrors.TypeInitializationError{TypeName: "FixedSizeBitfield", ErrorDetails: "Imported bitfield doesn't match the expected byte count."}
	}

	lastLegalBit := bf.bitCount % 8
	for i := lastLegalBit; i <= 7; i++ {
		bitState := (data[expectedByteCount-1]>>(7-i))&1 == 1
		if bitState {
			return &generalerrors.TypeInitializationError{TypeName: "FixedSizeBitfield", ErrorDetails: "Imported bitfield has extra bits set."}
		}
	}

	bf.internalField = data
	return nil
}

func (bf *FixedSizeBitfield) GetUnsetBitsIndices() []int {
	unsetBits := make([]int, 0)

	for byteind, currbyte := range bf.internalField {
		bitsToIgnore := 0
		if byteind == len(bf.internalField)-1 {
			bitsToIgnore = len(bf.internalField)*8 - bf.bitCount
		}

		for bitToTransfer := 0; bitToTransfer <= 7-bitsToIgnore; bitToTransfer++ {
			selectedBit := (currbyte >> (7 - bitToTransfer)) & 1
			if selectedBit == 0 {
				unsetBits = append(unsetBits, byteind*8+bitToTransfer)
			}
		}
	}

	return unsetBits
}

func NewFixedSizeBitfield(bitCount int) (*FixedSizeBitfield, error) {
	if bitCount <= 0 {
		return nil, &generalerrors.TypeInitializationError{TypeName: "FixedSizeBitfield", ErrorDetails: "Bitfield length cannot be 0 or negative."}
	}

	numBytes := (bitCount + 7) / 8
	return &FixedSizeBitfield{internalField: make([]byte, numBytes), bitCount: bitCount, bitsSet: 0}, nil
}
