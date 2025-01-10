package Bytesources

type ByteSource interface {
	Peek() (byte, error)
	Read() (byte, error)
	ReadMultiple(byteCount int) ([]byte, error)
}
