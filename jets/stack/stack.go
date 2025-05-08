package stack

// Simple stack component

type Stack[T any] struct{
	stack []any
}

func NewStack[T any](reserve int) *Stack[T] {
	return &Stack[T]{
		stack: make([]any, 0, reserve),
	}
}

// IsEmpty checks if the stack is empty
func (s *Stack[T]) IsEmpty() bool {
	return len(s.stack) == 0
}

// Push adds an element to the top of the stack
func (s *Stack[T]) Push(item *T) {
	s.stack = append(s.stack, item)
}

// Pop removes and returns the top element of the stack
func (s *Stack[T]) Pop() (*T, bool) {
	if s.IsEmpty() {
		return nil, false
	} else {
		index := len(s.stack) - 1
		element := s.stack[index]
		s.stack = s.stack[:index]
		// if element == nil {
		// 	return nil, true
		// }
		return element.(*T), true
	}
}

// Peek returns the top element of the stack without removing it
func (s *Stack[T]) Peek() (*T, bool) {
	if s.IsEmpty() {
		return nil, false
	} else {
		index := len(s.stack) - 1
		return s.stack[index].(*T), true
	}
}
