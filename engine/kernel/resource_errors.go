package kernel

import "fmt"

type ResourceErrorKind uint8

const (
	ResourceErrorUnknown ResourceErrorKind = iota + 1
	ResourceErrorUndeclaredRead
	ResourceErrorUndeclaredWrite
	ResourceErrorNotCollection
	ResourceErrorTypeMismatch
)

type ResourceError struct {
	Kind     ResourceErrorKind
	Resource ResourceID
	Expected ResourceKind
	Actual   ResourceKind
}

func (e *ResourceError) Error() string {
	switch e.Kind {
	case ResourceErrorUnknown:
		return fmt.Sprintf("unknown resource %s", e.Resource)
	case ResourceErrorUndeclaredRead:
		return fmt.Sprintf("undeclared read of %s", e.Resource)
	case ResourceErrorUndeclaredWrite:
		return fmt.Sprintf("undeclared write of %s", e.Resource)
	case ResourceErrorNotCollection:
		return fmt.Sprintf("resource %s is not a collection", e.Resource)
	case ResourceErrorTypeMismatch:
		return fmt.Sprintf("resource %s expects %s, got %s", e.Resource, e.Expected, e.Actual)
	default:
		return "unknown resource error"
	}
}
