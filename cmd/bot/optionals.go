package main

type optional[T any] struct {
	val     T
	isEmpty bool
}

func (o optional[T]) IsEmpty() bool {
	return o.isEmpty
}

func (o optional[T]) Get() T {
	return o.val
}

func Optional[T any](val T) optional[T] {
	return optional[T]{
		val: val,
	}
}

func EmptyOptional[T any]() optional[T] {
	return optional[T]{
		isEmpty: true,
	}
}
