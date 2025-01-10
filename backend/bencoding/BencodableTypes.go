package bencoding

type BencodableValue interface{}

type BencodableInt = int
type BencodableString = string
type BencodableList = []any
type BencodableMap = map[string]any
