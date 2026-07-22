package plush

import "fmt"

var punch_hole_constant = "<PLUSH_HOLE_%d>"

type HoleMarker struct {
	marker_name string
	input       string
	start, end  int
	content     string
	err         error
}

func PunchHoleMarkerName(index int) string {
	return fmt.Sprintf(punch_hole_constant, index)
}

func NewHoleMarker(markerName, input string, start, end int) HoleMarker {
	return HoleMarker{
		marker_name: markerName,
		input:       input,
		start:       start,
		end:         end,
		content:     "",
		err:         nil,
	}
}

func (h HoleMarker) MarkerName() string {
	return h.marker_name
}

func (h HoleMarker) Input() string {
	return h.input
}

func (h HoleMarker) Start() int {
	return h.start
}

func (h HoleMarker) End() int {
	return h.end
}

func (h HoleMarker) Content() string {
	return h.content
}

func (h HoleMarker) Err() error {
	return h.err
}
