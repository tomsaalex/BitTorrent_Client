package Bencoding

type BencodedValue interface{}

type BencodedInt struct {
	IntValue int
}

type BencodedString struct {
	StringValue string
}

type BencodedList struct {
	ListValue []BencodedValue
}

type BencodedMap struct {
	MapValue map[string]BencodedValue
}
