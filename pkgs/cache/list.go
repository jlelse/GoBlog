package cache

import (
	"sync"
)

type element[K comparable, V any] struct {
	prev, next *element[K, V]
	value      V
}

type list[K comparable, V any] struct {
	head, tail *element[K, V]
	mu         sync.RWMutex
}

func newList[K comparable, V any]() *list[K, V] {
	return &list[K, V]{}
}

func (l *list[K, V]) back() *element[K, V] {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tail
}

func (l *list[K, V]) pushFront(value V) *element[K, V] {
	l.mu.Lock()
	defer l.mu.Unlock()
	node := &element[K, V]{value: value}
	if l.head == nil {
		l.head = node
		l.tail = node
	} else {
		node.next = l.head
		l.head.prev = node
		l.head = node
	}
	return node
}

func (l *list[K, V]) moveToFront(node *element[K, V]) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.head == node {
		return
	}
	l.removeWithoutLock(node)
	node.next = l.head
	node.prev = nil
	l.head.prev = node
	l.head = node
}

func (l *list[K, V]) remove(node *element[K, V]) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.removeWithoutLock(node)
}

func (l *list[K, V]) removeWithoutLock(node *element[K, V]) {
	if node == l.head {
		l.head = node.next
	}
	if node == l.tail {
		l.tail = node.prev
	}
	if node.prev != nil {
		node.prev.next = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	}
	node.prev = nil
	node.next = nil
}
