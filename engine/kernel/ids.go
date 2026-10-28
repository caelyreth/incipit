package kernel

import "fmt"

// RowID identifies an item in a collection resource.
type RowID uint64

func NewRowID(value uint64) RowID { return RowID(value) }

func (id RowID) Value() uint64 { return uint64(id) }

func (id RowID) String() string { return fmt.Sprintf("row-%d", id) }
