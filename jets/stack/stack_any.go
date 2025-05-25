package stack

// Simple stack component

type StackAny struct{
	stack []any
}

func NewStackAny(reserve int) *StackAny {
	return &StackAny {
		stack: make([]any, 0, reserve),
	}
}

// IsEmpty checks if the stack is empty
func (s *StackAny) IsEmpty() bool {
	return len(s.stack) == 0
}

func (s *StackAny) Len() int {
	return len(s.stack)
}

// Push adds an element to the top of the stack
func (s *StackAny) Push(item any) {
	s.stack = append(s.stack, item)
}

// Pop removes and returns the top element of the stack
func (s *StackAny) Pop() (any, bool) {
	if s.IsEmpty() {
		return nil, false
	} else {
		index := len(s.stack) - 1
		element := s.stack[index]
		s.stack = s.stack[:index]
		return element, true
	}
}

// Peek returns the top element of the stack without removing it
func (s *StackAny) Peek() (any, bool) {
	if s.IsEmpty() {
		return nil, false
	} else {
		index := len(s.stack) - 1
		return s.stack[index], true
	}
}
