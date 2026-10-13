package random

import "unicode/utf16"

func stringToUTF16(value string) []uint16 { return utf16.Encode([]rune(value)) }
