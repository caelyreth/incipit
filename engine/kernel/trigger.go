package kernel

import (
	"fmt"
	"strings"
)

const stateWildcard = "state:*"

// TriggerKind identifies an event-trigger pattern.
type TriggerKind uint8

const (
	TriggerExact TriggerKind = iota + 1
	TriggerPrefix
	TriggerStateWildcard
)

// Trigger matches a rule against event keys. StateWildcard marks observers and
// never matches an ordinary key.
type Trigger struct {
	kind    TriggerKind
	pattern string
}

// ParseTrigger accepts an exact key, a prefix ending in one '*', or "state:*".
func ParseTrigger(pattern string) Trigger {
	switch {
	case pattern == stateWildcard:
		return Trigger{kind: TriggerStateWildcard, pattern: pattern}
	case strings.HasSuffix(pattern, "*"):
		return Trigger{kind: TriggerPrefix, pattern: strings.TrimSuffix(pattern, "*")}
	default:
		return Trigger{kind: TriggerExact, pattern: pattern}
	}
}

// TryParseTrigger validates a declaration before linking it.
func TryParseTrigger(pattern string) (Trigger, error) {
	trigger := ParseTrigger(pattern)
	if err := trigger.Validate(); err != nil {
		return Trigger{}, err
	}
	return trigger, nil
}

func NewExactTrigger(key string) Trigger {
	return Trigger{kind: TriggerExact, pattern: key}
}

func NewPrefixTrigger(prefix string) Trigger {
	return Trigger{kind: TriggerPrefix, pattern: prefix}
}

func NewStateWildcardTrigger() Trigger {
	return Trigger{kind: TriggerStateWildcard, pattern: stateWildcard}
}

func (t Trigger) Kind() TriggerKind { return t.kind }

func (t Trigger) Pattern() string { return t.pattern }

func (t Trigger) Validate() error {
	switch t.kind {
	case TriggerStateWildcard:
		if t.pattern != stateWildcard {
			return fmt.Errorf("invalid state wildcard trigger %q", t.pattern)
		}
	case TriggerExact:
		if t.pattern == "" || strings.Contains(t.pattern, "*") {
			return fmt.Errorf("%s", t.String())
		}
	case TriggerPrefix:
		if t.pattern == "state:" || strings.Contains(t.pattern, "*") {
			return fmt.Errorf("%s", t.String())
		}
	default:
		return fmt.Errorf("unknown trigger kind %d", t.kind)
	}
	return nil
}

func (t Trigger) Matches(key string) bool {
	switch t.kind {
	case TriggerExact:
		return t.pattern == key
	case TriggerPrefix:
		return strings.HasPrefix(key, t.pattern)
	default:
		return false
	}
}

func (t Trigger) Overlaps(other Trigger) bool {
	switch {
	case t.kind == TriggerStateWildcard && other.kind == TriggerStateWildcard:
		return true
	case t.kind == TriggerStateWildcard || other.kind == TriggerStateWildcard:
		return false
	case t.kind == TriggerExact && other.kind == TriggerExact:
		return t.pattern == other.pattern
	case t.kind == TriggerExact && other.kind == TriggerPrefix:
		return strings.HasPrefix(t.pattern, other.pattern)
	case t.kind == TriggerPrefix && other.kind == TriggerExact:
		return strings.HasPrefix(other.pattern, t.pattern)
	case t.kind == TriggerPrefix && other.kind == TriggerPrefix:
		return strings.HasPrefix(t.pattern, other.pattern) || strings.HasPrefix(other.pattern, t.pattern)
	default:
		return false
	}
}

func (t Trigger) String() string {
	switch t.kind {
	case TriggerStateWildcard:
		return stateWildcard
	case TriggerPrefix:
		return t.pattern + "*"
	default:
		return t.pattern
	}
}
