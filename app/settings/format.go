package settings

import "strconv"

func uintString(value uint64) string {
	return strconv.FormatUint(value, 10)
}

func intString(value int) string {
	return strconv.Itoa(value)
}
