package collection

type Node[T any] struct {
	data     T
	previous *Node[T]
	next     *Node[T]
}

func (n *Node[T]) Data() T {
	return n.data
}

func (n *Node[T]) SetData(value T) {
	n.data = value
}

func (n *Node[T]) Next() *Node[T] {
	return n.next
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
		d.tail.next = n
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
		n.previous.next = n.next
	} else {
		d.Head = n.next
	}
	if n.next != nil {
		n.next.previous = n.previous
	} else {
		d.tail = n.previous
	}

	n.previous, n.next = nil, nil
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
