package plush_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type parityEchoName string

func (n parityEchoName) Echo() string {
	return string(n)
}

func (n parityEchoName) String() string {
	return string(n)
}

type parityRobot struct {
	Name parityEchoName
}

type parityStringRobot struct {
	Name string
}

type parityAvatar string

func (a parityAvatar) URL() string {
	return string(a)
}

type parityRobotProfile struct {
	Names []parityEchoName
}

type parityRobotStats struct {
	Label string
}

type parityNestedRobot struct {
	Name     parityEchoName
	Avatar   parityAvatar
	Profile  parityRobotProfile
	Stats    []parityRobotStats
	Metadata map[string]parityRobot
	Friends  []parityRobot
}

func (r parityNestedRobot) GetFriends() []parityRobot {
	return r.Friends
}

type parityAccessNode struct {
	N    string
	Next *parityAccessNode
}

type parityAccessRobot struct {
	Name    parityEchoName
	Avatar  parityAvatar
	Next    *parityAccessNode
	Friends []parityAccessRobot
	Map     map[string]parityAccessRobot
	Nested  parityAccessNested
}

func (r parityAccessRobot) GetFriends() []parityAccessRobot {
	return r.Friends
}

type parityAccessNested struct {
	Map map[string]parityAccessBucket
}

type parityAccessBucket struct {
	Items []parityAccessRobot
}

type parityRobotFactory struct {
	values []parityAccessRobot
}

func (f parityRobotFactory) Robots() []parityAccessRobot {
	return f.values
}

type parityFunctionReturnFactory struct {
	robot parityFunctionReturnRobot
}

func (f parityFunctionReturnFactory) Robots() parityFunctionReturnRobot {
	return f.robot
}

type parityFunctionReturnRobot struct {
	name parityEchoName
}

func (r parityFunctionReturnRobot) Name() parityEchoName {
	return r.name
}

type parityPointerMethodRobot struct {
	name string
}

func (r *parityPointerMethodRobot) Name() string {
	return r.name
}

type parityArrayHolder struct {
	Items [2]parityAccessRobot
}

type parityAccessPerson struct {
	born time.Time
}

func (p parityAccessPerson) GetBorn() time.Time {
	return p.born
}

func Test_Parity_Struct_Access_Field(t *testing.T) {
	compareRender(t, `<%= robot.Name %>`, contextWith(map[string]interface{}{
		"robot": parityStringRobot{Name: "bender"},
	}))
}

func Test_Parity_Struct_Access_Nested_Method(t *testing.T) {
	compareRender(t, `<%= robot.Name.Echo() %>`, contextWith(map[string]interface{}{
		"robot": parityRobot{Name: "bender"},
	}))
}

func Test_Parity_Struct_Access_Slice_Start(t *testing.T) {
	compareVMRender(t, `<%= robots[0].Name.Echo() %>`, "bender", contextWith(map[string]interface{}{
		"robots": []parityRobot{{Name: "bender"}},
	}))
}

func Test_Parity_Struct_Access_Nested_Field(t *testing.T) {
	compareVMRender(t, `<%= robot.Profile.Names[1].Echo() %>`, "flexo", contextWith(map[string]interface{}{
		"robot": parityNestedRobot{
			Profile: parityRobotProfile{Names: []parityEchoName{"bender", "flexo"}},
		},
	}))
}

func Test_Parity_Struct_Access_Nested_Slice_Field(t *testing.T) {
	compareRender(t, `<%= robots[0].Stats[1].Label %>`, contextWith(map[string]interface{}{
		"robots": []parityNestedRobot{{
			Stats: []parityRobotStats{{Label: "first"}, {Label: "second"}},
		}},
	}))
}

func Test_Parity_Struct_Access_Nested_Map_Slice_Method(t *testing.T) {
	compareVMRender(t, `<%= robot.Metadata["owner"].Name.Echo() %>`, "fry", contextWith(map[string]interface{}{
		"robot": parityNestedRobot{
			Metadata: map[string]parityRobot{
				"owner": {Name: "fry"},
			},
		},
	}))
}

func Test_Parity_Struct_Access_Typed_Map_String_Index(t *testing.T) {
	requireBothRender(t, `<%= labels["status"] %>|<%= counts["active"] %>|<%= robots["bender"].Name %>`, `&lt;ok&gt;|7|Bender`, contextWith(map[string]interface{}{
		"labels": map[string]string{"status": "<ok>"},
		"counts": map[string]uint32{"active": 7},
		"robots": map[string]parityStringRobot{
			"bender": {Name: "Bender"},
		},
	}))
}

func Test_Parity_Struct_Access_Typed_Interface_Map_String_Index(t *testing.T) {
	requireBothRender(t, `<%= labels["status"] %>`, `&lt;ok&gt;`, contextWith(map[string]interface{}{
		"labels": map[interface{}]string{"status": "<ok>"},
	}))
}

func Test_Parity_Struct_Access_Method_Return_Index(t *testing.T) {
	compareVMRender(t, `<%= robot.GetFriends()[0].Name.Echo() %>`, "leela", contextWith(map[string]interface{}{
		"robot": parityNestedRobot{
			Friends: []parityRobot{{Name: "leela"}},
		},
	}))
}

func Test_Parity_Struct_Access_Method_On_Nested_Field(t *testing.T) {
	compareRender(t, `<%= robot.Avatar.URL() %>`, contextWith(map[string]interface{}{
		"robot": parityNestedRobot{
			Avatar: "bender.jpg",
		},
	}))
}

func Test_Parity_Struct_Access_Method_Return_Native_Chain(t *testing.T) {
	requireBothRender(t, `<%= nour.GetBorn().Format("Jan 2, 2006") %>`, "Jan 11, 1993", contextWith(map[string]interface{}{
		"nour": parityAccessPerson{
			born: time.Date(1993, time.January, 11, 0, 0, 0, 0, time.UTC),
		},
	}))
}

func Test_Parity_Struct_Access_Nested_Function_Returns(t *testing.T) {
	compareVMRender(t, `<%= factory().Robots().Name().Echo() %>`, "nested", contextWith(map[string]interface{}{
		"factory": func() parityFunctionReturnFactory {
			return parityFunctionReturnFactory{
				robot: parityFunctionReturnRobot{name: "nested"},
			}
		},
	}))
}

func Test_Parity_Struct_Access_Phase_5_Matrix(t *testing.T) {
	robot := parityAccessRobot{
		Name:   "bender",
		Avatar: "bender.jpg",
		Next: &parityAccessNode{
			N:    "one",
			Next: &parityAccessNode{N: "two"},
		},
		Friends: []parityAccessRobot{
			{Name: "fry"},
			{Name: "leela"},
		},
		Map: map[string]parityAccessRobot{
			"owner": {Name: "fry"},
		},
		Nested: parityAccessNested{
			Map: map[string]parityAccessBucket{
				"team": {Items: []parityAccessRobot{{Name: "amy"}}},
			},
		},
	}
	robots := []parityAccessRobot{robot}

	ctx := contextWith(map[string]interface{}{
		"robot":     robot,
		"robots":    robots,
		"key":       "owner",
		"nestedKey": "team",
		"getRobot": func() parityAccessRobot {
			return robot
		},
		"getRobots": func() []parityAccessRobot {
			return []parityAccessRobot{{Name: "scruffy"}}
		},
		"factory": func() parityRobotFactory {
			return parityRobotFactory{values: []parityAccessRobot{{Name: "factory"}}}
		},
	})

	tests := []struct {
		input    string
		expected string
	}{
		{`<%= robot.Name %>`, "bender"},
		{`<%= robot.Name.Echo() %>`, "bender"},
		{`<%= robot.Avatar.URL() %>`, "bender.jpg"},
		{`<%= robot.Next.N %>`, "one"},
		{`<%= robot.Next.Next.N %>`, "two"},
		{`<%= robots[0].Name %>`, "bender"},
		{`<%= robots[0].Name.Echo() %>`, "bender"},
		{`<%= robots[0].Avatar.URL() %>`, "bender.jpg"},
		{`<%= robots[0].Friends[1].Name %>`, "leela"},
		{`<%= robots[0].Friends[1].Name.Echo() %>`, "leela"},
		{`<%= robot.Map[key].Name %>`, "fry"},
		{`<%= robot.Map[key].Name.Echo() %>`, "fry"},
		{`<%= robot.Nested.Map[nestedKey].Items[0].Name %>`, "amy"},
		{`<%= robot.Nested.Map[nestedKey].Items[0].Name.Echo() %>`, "amy"},
		{`<%= getRobot().Name %>`, "bender"},
		{`<%= getRobot().Name.Echo() %>`, "bender"},
		{`<%= getRobots()[0].Name %>`, "scruffy"},
		{`<%= getRobots()[0].Name.Echo() %>`, "scruffy"},
		{`<%= robot.GetFriends()[0].Name %>`, "fry"},
		{`<%= robot.GetFriends()[0].Name.Echo() %>`, "fry"},
		{`<%= factory().Robots()[0].Name.Echo() %>`, "factory"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			compareVMRender(t, tt.input, tt.expected, ctx)
		})
	}
}

func Test_Parity_Struct_Access_Reflection_Compatibility(t *testing.T) {
	ctx := contextWith(map[string]interface{}{
		"structRobot": parityPointerMethodRobot{name: "struct"},
		"ptrRobot":    &parityPointerMethodRobot{name: "ptr"},
		"nilRobot": parityAccessRobot{
			Next: nil,
			Map:  map[string]parityAccessRobot{},
		},
	})

	tests := []struct {
		input    string
		expected string
	}{
		{`<%= structRobot.Name() %>`, "struct"},
		{`<%= ptrRobot.Name() %>`, "ptr"},
		{`<%= nilRobot.Next.N %>`, ""},
		{`<%= nilRobot.Map["missing"].Name %>`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			requireBothRender(t, tt.input, tt.expected, ctx)
		})
	}
}

func Test_Parity_Struct_Access_Typed_Collections(t *testing.T) {
	ctx := contextWith(map[string]interface{}{
		"holder": parityArrayHolder{
			Items: [2]parityAccessRobot{{Name: "zero"}, {Name: "one"}},
		},
		"lookup": map[int]string{1: "one"},
		"items":  []parityAccessRobot{{Name: "zero"}},
	})

	compareVMRender(t, `<%= holder.Items[1].Name %>`, "one", ctx)
	compareBothRenderError(t, `<%= lookup["bad"] %>`, ctx)
	compareBothRenderError(t, `<%= items["bad"].Name %>`, ctx)
	compareBothRenderError(t, `<%= items[2].Name %>`, ctx)
}

func Test_Parity_Struct_Access_Errors(t *testing.T) {
	type privatePeople struct {
		FirstName string
	}
	type privateEmployee struct {
		employee []privatePeople
	}

	ctx := contextWith(map[string]interface{}{
		"departments": map[string]privateEmployee{
			"HR": {employee: []privatePeople{{FirstName: "John"}}},
		},
		"robot": parityAccessRobot{
			Name:    "bender",
			Friends: []parityAccessRobot{{Name: "fry"}},
		},
	})

	compareBothRenderError(t, `<%= departments["HR"].employee[0].FirstName %>`, ctx)
	compareBothRenderError(t, `<%= robot.Missing %>`, ctx)
	compareBothRenderError(t, `<%= robot.Missing() %>`, ctx)
	compareBothRenderError(t, `<%= robot.Name() %>`, ctx)
	compareBothRenderError(t, `<%= robot.Name.Missing() %>`, ctx)
	compareBothRenderError(t, `<%= robot.GetFriends()[9].Name %>`, ctx)
}

func compareVMRender(t *testing.T, input, expected string, factory contextFactory) {
	t.Helper()

	out, err := renderVM(input, factory)
	require.NoError(t, err)
	require.Equal(t, expected, out)
}
