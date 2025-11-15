package collection

type Node[T any] struct {
	data     T
	previous *Node[T]
	Next     *Node[T]
}

func (n *Node[T]) Data() T {
	return n.data
}

type DoubleLinkedList[T any] struct {
	Head  *Node[T]
	tail  *Node[T]
	count int
}

func (d *DoubleLinkedList[T]) Add(value T) *Node[T] {
	n := &Node[T]{data: value}
	if d.Head == nil {
		d.Head, d.tail = n, n
	} else {
		n.previous = d.tail
		d.tail.Next = n
		d.tail = n
	}

	d.count++
	return n
}

func (d *DoubleLinkedList[T]) Remove(n *Node[T]) {
	if n == nil {
		return
	}
	if n.previous != nil {
		n.previous.Next = n.Next
	} else {
		d.Head = n.Next
	}
	if n.Next != nil {
		n.Next.previous = n.previous
	} else {
		d.tail = n.previous
	}

	n.previous, n.Next = nil, nil
	d.count--
}

func (d *DoubleLinkedList[T]) First() (T, bool) {
	if d.Head == nil {
		var zero T
		return zero, false
	}

	return d.Head.data, true
}

func (d *DoubleLinkedList[T]) Last() (T, bool) {
	if d.tail == nil {
		var zero T
		return zero, false
	}

	return d.tail.data, true
}

func (d *DoubleLinkedList[T]) Count() int {
	return d.count
}
