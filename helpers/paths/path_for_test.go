package paths

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/require"
)

type Car struct {
	ID int
}

type boat struct {
	Slug string
}

type plane struct{}

func (plane) ToParam() string {
	return "aeroplane"
}

type truck struct{}

func (truck) ToPath() string {
	return "/a/truck"
}

type BadCar struct {
	IDs int
}

func Test_PathFor(t *testing.T) {
	table := []struct {
		in  interface{}
		out string
		err bool
	}{
		{Car{1}, "/cars/1", false},
		{Car{}, "/cars", false},
		{&Car{}, "/cars", false},
		{boat{"titanic"}, "/boats/titanic", false},
		{plane{}, "/planes/aeroplane", false},
		{truck{}, "/a/truck", false},
		{[]interface{}{truck{}, plane{}}, "/a/truck/planes/aeroplane", false},
		{"foo", "/foo", false},
		{template.HTML("foo"), "/foo", false},
		{map[int]int{}, "", true},
		{nil, "", true},
		{[]interface{}{truck{}, nil}, "", true},
		{BadCar{}, "", true},
		{"https://www.google.com", "https://www.google.com", false},
	}

	for _, tt := range table {
		t.Run(tt.out, func(st *testing.T) {
			r := require.New(st)
			s, err := PathFor(tt.in)
			if tt.err {
				r.Error(err)
				return
			}
			r.NoError(err)
			r.Equal(tt.out, s)
		})
	}
}
