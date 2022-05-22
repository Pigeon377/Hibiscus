package disk

import "os"

const (
	blockSize = 4096 / 8

	fillByte      = 2
	byteSep       = 3
	byteSepString = string(rune(byteSep))
)

func init() {
	os.Mkdir("data", 666)
}