package plush

import (
	"fmt"
	"reflect"
)

// Iterator type can be implemented and used by the `for` command to build loops in templates
type Iterator interface {
	Next() interface{}
}

func GroupByHelper(size int, underlying interface{}) (*groupBy, error) {
	if size <= 0 {
		return nil, fmt.Errorf("size must be greater than zero")
	}

	u := reflect.Indirect(reflect.ValueOf(underlying))
	group := []reflect.Value{}

	switch u.Kind() {
	case reflect.Array, reflect.Slice:
		if u.Len() == size {
			return &groupBy{
				group: []reflect.Value{u},
			}, nil
		}

		groupSize := u.Len() / size
		if u.Len()%size != 0 {
			groupSize++
		}

		pos := 0
		for pos < u.Len() {
			e := pos + groupSize
			if e > u.Len() {
				e = u.Len()
			}
			group = append(group, u.Slice(pos, e))
			pos += groupSize
		}
	default:
		return nil, fmt.Errorf("can not use %T in groupBy", underlying)
	}

	g := &groupBy{
		group: group,
	}
	return g, nil
}

type groupBy struct {
	pos   int
	group []reflect.Value
}

func (g *groupBy) Next() interface{} {
	if g.pos >= len(g.group) {
		return nil
	}

	v := g.group[g.pos]
	g.pos++

	return v.Interface()
}
