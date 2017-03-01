package plush

// Iterator can be defined and passed to a `for` call.
// If `Next()` returns `nil` the iteration stops.
type Iterator interface {
	Next() interface{}
}

type ranger struct {
	pos int
	end int
}

func (r *ranger) Next() interface{} {
	if r.pos < r.end {
		r.pos++
		return r.pos
	}
	return nil
}

func rangeHelper(a, b int) Iterator {
	return &ranger{pos: a - 1, end: b}
}

func betweenHelper(a, b int) Iterator {
	return &ranger{pos: a, end: b - 1}
}

func untilHelper(a int) Iterator {
	return &ranger{pos: -1, end: a - 1}
}
