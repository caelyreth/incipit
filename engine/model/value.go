package model

import "strconv"

type attributeKind uint8

const (
	attributeNull attributeKind = iota
	attributeBool
	attributeInteger
	attributeText
)

// AttributeValue is a small, owned value in the semantic attribute vocabulary.
//
// The zero value is Null. Values are immutable and directly comparable.
type AttributeValue struct {
	kind    attributeKind
	boolean bool
	integer int64
	text    string
}

// NullValue returns the null attribute value.
func NullValue() AttributeValue {
	return AttributeValue{}
}

// BoolValue returns a boolean attribute value.
func BoolValue(value bool) AttributeValue {
	return AttributeValue{kind: attributeBool, boolean: value}
}

// IntegerValue returns a signed integer attribute value.
func IntegerValue(value int64) AttributeValue {
	return AttributeValue{kind: attributeInteger, integer: value}
}

// TextValue returns a text attribute value.
func TextValue(value string) AttributeValue {
	return AttributeValue{kind: attributeText, text: value}
}

// Kind reports the value's vocabulary kind.
func (v AttributeValue) Kind() AttributeKind {
	return AttributeKind(v.kind)
}

// Bool returns the boolean payload and whether the value is boolean.
func (v AttributeValue) Bool() (bool, bool) {
	return v.boolean, v.kind == attributeBool
}

// Integer returns the integer payload and whether the value is an integer.
func (v AttributeValue) Integer() (int64, bool) {
	return v.integer, v.kind == attributeInteger
}

// Text returns the text payload and whether the value is text.
func (v AttributeValue) Text() (string, bool) {
	return v.text, v.kind == attributeText
}

// AttributeKind is the public discriminator for AttributeValue.
type AttributeKind uint8

const (
	AttributeNull AttributeKind = iota
	AttributeBool
	AttributeInteger
	AttributeText
)

func (v AttributeValue) String() string {
	switch v.kind {
	case attributeBool:
		return strconv.FormatBool(v.boolean)
	case attributeInteger:
		return strconv.FormatInt(v.integer, 10)
	case attributeText:
		return v.text
	default:
		return "null"
	}
}
