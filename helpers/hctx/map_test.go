package hctx

import (
	"reflect"
	"testing"
)

func TestMerge(t *testing.T) {
	map1 := Map{
		"Test": 1,
		"Take": 2,
	}
	map2 := Map{
		"Testing": '1',
		"Taking":  "2",
	}
	mapM := Map{
		"Test":    1,
		"Take":    2,
		"Testing": '1',
		"Taking":  "2",
	}
	tests := []struct {
		name string
		maps []Map
		want Map
	}{
		{"good single", []Map{map1}, map1},
		{"good together", []Map{map1, map2}, mapM},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Merge(tt.maps...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Merge() = %v, want %v", got, tt.want)
			}
		})
	}
}
