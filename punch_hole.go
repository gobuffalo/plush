package plush

var punch_hole_constant = "<PLUSH_HOLE_%d>"

type HoleMarker struct {
	marker_name string
	input       string
	start, end  int
	content     string
	err         error
}
