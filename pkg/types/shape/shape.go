package shape

type Kind int

const (
	KindDirect Kind = iota
	KindPointer
	KindSlice
	KindArray
	KindMapValue
	KindMapKey
)

type Shape struct {
	Param string
	Kind  Kind
}
