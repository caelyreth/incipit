package kernel

const ProjectedTextKey = "editor:projected-text"

type EventKind uint8

const (
	EventCommand EventKind = iota + 1
	EventText
	EventAttribute
	EventProjected
	EventSpan
)

// Event is a typed command or checked input request. Text payload belongs to
// the event, not its lookup key.
type Event struct {
	kind      EventKind
	key       string
	text      *TextInput
	attr      *AttributeInput
	projected *ProjectedEditInput
	span      *SpanInput
	origin    *EventOrigin
	scopes    []ScopeKind
}

// WithScopes is the event standing in scopes, innermost first: rules of an
// inner scope answer it before an outer one's.
func (e Event) WithScopes(scopes ...ScopeKind) Event {
	e.scopes = append([]ScopeKind(nil), scopes...)
	return e
}

// Scopes are the scopes the event stands in, innermost first, the page
// left implicit.
func (e Event) Scopes() []ScopeKind { return e.scopes }

func NewCommandEvent(key string) Event {
	return Event{kind: EventCommand, key: key}
}

func NewTextEvent(input TextInput, origin EventOrigin) Event {
	return Event{kind: EventText, key: TextInputKey, text: &input, origin: &origin}
}

func NewAttributeEvent(input AttributeInput, origin EventOrigin) Event {
	return Event{kind: EventAttribute, key: AttributeInputKey, attr: &input, origin: &origin}
}

// NewSpanEvent requests a replacement across nodes.
func NewSpanEvent(input SpanInput, origin EventOrigin) Event {
	return Event{kind: EventSpan, key: SpanInputKey, span: &input, origin: &origin}
}

func NewProjectedEvent(input ProjectedEditInput, origin EventOrigin) Event {
	key := ProjectedTextKey
	if input.Interactive() {
		key = TextInputKey
	}
	return Event{kind: EventProjected, key: key, projected: &input, origin: &origin}
}

func NewProjectedTextEvent(input TextInput, interactive bool, origin EventOrigin) Event {
	return NewProjectedEvent(NewProjectedTextInput(input, interactive), origin)
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
	if e.text != nil {
		return *e.text, true
	}
	if e.projected != nil {
		return e.projected.input, true
	}
	return TextInput{}, false
}

func (e Event) AttributeInput() (AttributeInput, bool) {
	if e.attr == nil {
		return AttributeInput{}, false
	}
	return *e.attr, true
}

func (e Event) SpanInput() (SpanInput, bool) {
	if e.span == nil {
		return SpanInput{}, false
	}
	return *e.span, true
}

func (e Event) ProjectedEdit() (ProjectedEditInput, bool) {
	if e.projected == nil {
		return ProjectedEditInput{}, false
	}
	return e.projected.Clone(), true
}

func (e Event) ProjectedEditPayload() *ProjectedEditInput {
	return e.projected
}

func (e Event) HasReservedCommandKey() bool {
	return e.kind == EventCommand && (e.key == TextInputKey || e.key == AttributeInputKey || e.key == SpanInputKey)
}
