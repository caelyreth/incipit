package kernel

type EventKind uint8

const (
	EventCommand EventKind = iota + 1
	EventText
	EventAttribute
)

// Event is a typed command or checked input request. Text payload belongs to
// the event, not its lookup key.
type Event struct {
	kind   EventKind
	key    string
	text   *TextInput
	attr   *AttributeInput
	origin *EventOrigin
}

func NewCommandEvent(key string) Event {
	return Event{kind: EventCommand, key: key}
}

func NewTextEvent(input TextInput, origin EventOrigin) Event {
	return Event{kind: EventText, key: TextInputKey, text: &input, origin: &origin}
}

func NewAttributeEvent(input AttributeInput, origin EventOrigin) Event {
	return Event{kind: EventAttribute, key: AttributeInputKey, attr: &input, origin: &origin}
}

func (e Event) Kind() EventKind { return e.kind }
func (e Event) Key() string     { return e.key }

func (e Event) Origin() (EventOrigin, bool) {
	if e.origin == nil {
		return 0, false
	}
	return *e.origin, true
}

func (e Event) WithOrigin(origin EventOrigin) Event {
	e.origin = &origin
	return e
}

func (e Event) TextInput() (TextInput, bool) {
	if e.text == nil {
		return TextInput{}, false
	}
	return *e.text, true
}

func (e Event) AttributeInput() (AttributeInput, bool) {
	if e.attr == nil {
		return AttributeInput{}, false
	}
	return *e.attr, true
}

func (e Event) HasReservedCommandKey() bool {
	return e.kind == EventCommand && (e.key == TextInputKey || e.key == AttributeInputKey)
}
