package kernel

// EventOrigin identifies how an event entered the editor policy system.
type EventOrigin uint8

const (
	EventOriginTyping EventOrigin = iota + 1
	EventOriginPaste
	EventOriginIME
	EventOriginSemantic
)

func (o EventOrigin) Matches(other EventOrigin) bool {
	return o == other
}

func (o EventOrigin) String() string {
	switch o {
	case EventOriginTyping:
		return "typing"
	case EventOriginPaste:
		return "paste"
	case EventOriginIME:
		return "ime"
	case EventOriginSemantic:
		return "semantic"
	default:
		return "unknown"
	}
}

func OriginMatches(declared []EventOrigin, actual EventOrigin) bool {
	if len(declared) == 0 {
		return true
	}
	for _, origin := range declared {
		if origin == actual {
			return true
		}
	}
	return false
}

func OriginsOverlap(first, second []EventOrigin) bool {
	if len(first) == 0 || len(second) == 0 {
		return true
	}
	for _, a := range first {
		for _, b := range second {
			if a == b {
				return true
			}
		}
	}
	return false
}
