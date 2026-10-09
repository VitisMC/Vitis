package nbt

// List is an NBT list tag containing elements of the same type.
type List struct {
	elementType byte
	elements    []interface{}
}

// NewList creates a new list with the given element tag type.
func NewList(elementType byte) *List {
	return &List{elementType: elementType}
}

// Clone returns a deep copy of the list.
func (l *List) Clone() *List {
	if l == nil {
		return nil
	}
	clone := NewList(l.elementType)
	clone.elements = make([]interface{}, 0, len(l.elements))
	for _, element := range l.elements {
		clone.elements = append(clone.elements, cloneValue(element))
	}
	return clone
}

// Add appends an element to the list.
func (l *List) Add(v interface{}) *List {
	l.elements = append(l.elements, v)
	return l
}

// ElementType returns the tag type of list elements.
func (l *List) ElementType() byte {
	return l.elementType
}

// Elements returns the list elements.
func (l *List) Elements() []interface{} {
	return l.elements
}

// Len returns the number of elements.
func (l *List) Len() int {
	return len(l.elements)
}
