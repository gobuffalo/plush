package vm

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/vm/compiler"
	"github.com/gobuffalo/tags/v3"
	"github.com/stretchr/testify/require"
)

type fastScriptPlanMenu struct {
	Items []fastScriptPlanItem
}

type fastScriptPlanItem struct {
	Name  string
	Count int
}

type fastScriptPlanPair struct {
	Key   string
	Value string
}

type fastScriptPlanRecord struct {
	ID int
}

type fastScriptPlanChoiceSet struct {
	Entries []fastScriptPlanChoice
}

type fastScriptPlanChoice struct {
	ID    string
	Label string
}

type fastScriptPlanNodeSet struct {
	Nodes []fastScriptPlanNode
}

type fastScriptPlanNode struct {
	Label string
}

type fastScriptPlanRecursiveNode struct {
	Label    string
	Enabled  bool
	Children []fastScriptPlanRecursiveNode
}

type fastScriptPlanAssetSet struct {
	Assets []fastScriptPlanAsset
}

type fastScriptPlanAsset struct {
	URL string
}

type fastScriptPlanBuilder struct{}

type fastScriptPlanFormBuilder struct{}

type fastScriptPlanTagFormBuilder struct{}

type fastScriptPlanUser struct {
	Name string
}

type fastScriptPlanRenderBlock struct {
	Type    string
	BlockID string
}

func (u fastScriptPlanUser) Render(mode string) string {
	return mode + ":" + u.Name
}

func (fastScriptPlanBuilder) RenderControl(options map[string]interface{}) string {
	name, _ := options["name"].(string)
	typ, _ := options["type"].(string)
	return name + ":" + typ
}

func (fastScriptPlanBuilder) RenderSelect(name string, options map[string]interface{}) string {
	values, _ := options["options"].([]interface{})
	selected, _ := options["value"].(string)
	return name + ":" + fmt.Sprint(len(values)) + ":" + selected
}

func (fastScriptPlanBuilder) InputTag(name string) string {
	return "input:" + name
}

func (fastScriptPlanBuilder) SelectTag(name string, options map[string]interface{}) string {
	values, _ := options["options"].([]interface{})
	selected, _ := options["value"].(string)
	return "select:" + name + ":" + fmt.Sprint(len(values)) + ":" + selected
}

func (fastScriptPlanBuilder) CheckboxTag(name string, options map[string]interface{}) string {
	return "check:" + name + ":" + fmt.Sprint(options["unchecked"])
}

func (fastScriptPlanFormBuilder) InputTag(options map[string]interface{}) string {
	return fmt.Sprintf("input:%v:%v:%v", options["name"], options["value"], options["class"])
}

func (fastScriptPlanFormBuilder) SelectTag(options map[string]interface{}) string {
	return fmt.Sprintf("select:%v:%v:%v", options["name"], options["value"], options["options"])
}

func (fastScriptPlanTagFormBuilder) InputTag(options tags.Options) *tags.Tag {
	if options["type"] == nil {
		options["type"] = "text"
	}
	return tags.New("input", options)
}

func (fastScriptPlanTagFormBuilder) SelectTag(options tags.Options) template.HTML {
	return template.HTML(fmt.Sprintf(`<select name="%v"></select>`, options["name"]))
}

const fastScriptPlanNestedSelectionTemplate = `<% let selectedEntry = set.Entries[0] %><%= if(input["target_id"] != "" && count(set.Entries) > 0) { %><%= for (_, candidate) in set.Entries { %><%= if(candidate.ID == input["target_id"]) { %><% selectedEntry = candidate %><% } %><% } %><% } %><%= selectedEntry.Label %>`

func fastScriptPlanNestedSelectionContext() *plush.Context {
	return plush.NewContextWith(map[string]interface{}{
		"set": fastScriptPlanChoiceSet{Entries: []fastScriptPlanChoice{
			{ID: "first", Label: "First"},
			{ID: "second", Label: "Second"},
		}},
		"input": map[string]interface{}{"target_id": "second"},
		"count": func(values []fastScriptPlanChoice) int {
			return len(values)
		},
	})
}

func Test_VM_Fast_Render_Literal_Lets_And_Path_Loops(t *testing.T) {
	tmpl, err := Compile(`<% let title = "Default" %><title><%= title %></title><%= for (item) in menu.Items { %><%= item.Name %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"menu": fastScriptPlanMenu{Items: []fastScriptPlanItem{
			{Name: "One"},
			{Name: "Two"},
		}},
	}))
	require.NoError(t, err)
	require.Equal(t, `<title>Default</title>One;Two;`, out)
}

func Test_VM_Fast_Render_Loop_Let_With_Helper_And_Arithmetic_Arg(t *testing.T) {
	tmpl, err := Compile(`<%= for (_, item) in items { %><% let normalizedPath = replace(document.Path, "-draft", "", 0 - 1) %><%= normalizedPath %>:<%= item.Name %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"document": struct {
			Path string
		}{Path: "guide-draft"},
		"items": []fastScriptPlanItem{
			{Name: "One"},
			{Name: "Two"},
		},
		"replace": strings.Replace,
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `guide:One;guide:Two;`, out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
	require.Empty(t, diagnostics.FastReject)
}

func Test_VM_Fast_Render_Loop_Let_Spends_Assignment_Budget(t *testing.T) {
	tmpl, err := Compile(`<%= for (_, item) in items { %><% let label = item.Name %><%= label %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Empty(t, tmpl.bytecode.FastReject)

	costs := plush.ZeroCosts()
	costs.Assignment = 1
	budget := plush.NewBudgetWithCosts(1, costs)
	ctx := plush.NewContextWith(map[string]interface{}{
		"items": []fastScriptPlanItem{
			{Name: "One"},
			{Name: "Two"},
		},
	}).WithBudget(budget)

	_, err = tmpl.Render(ctx)
	require.ErrorIs(t, err, plush.ErrBudgetExceeded)
	require.Equal(t, int64(2), budget.Stats().Assignments)
}

func Test_VM_Fast_Render_Loop_Assignment_Updates_Outer_Binding(t *testing.T) {
	tmpl, err := Compile(`<% let last = "" %><%= for (_, item) in items { %><% let label = item %><% last = label %><%= label %><% } %>|<%= last %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	require.Len(t, tmpl.bytecode.FastRenderPlan.Segments, 4)
	require.Equal(t, compiler.FastLoopPartAssign, tmpl.bytecode.FastRenderPlan.Segments[1].Loop.Parts[1].Kind)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"items": []string{"a", "b"},
	}))
	require.NoError(t, err)
	require.Equal(t, "ab|b", out)

	out, err = tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"items": []string{},
	}))
	require.NoError(t, err)
	require.Equal(t, "|", out)
}

func Test_VM_Fast_Render_Nested_Conditional_Loop_Assignment_Updates_Outer_Binding(t *testing.T) {
	tmpl, err := Compile(fastScriptPlanNestedSelectionTemplate)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	out, err := tmpl.Render(fastScriptPlanNestedSelectionContext())
	require.NoError(t, err)
	require.Equal(t, "Second", out)
}

func Test_VM_Bytecode_Render_Nested_Conditional_Loop_Assignment_Updates_Outer_Binding(t *testing.T) {
	tmpl, err := Compile(fastScriptPlanNestedSelectionTemplate)
	require.NoError(t, err)

	out, err := renderBytecodeVMWithState(tmpl.bytecode, fastScriptPlanNestedSelectionContext(), "", false, "")
	require.NoError(t, err)
	require.Equal(t, "Second", out)
}

func Test_VM_Fast_Render_Ignores_Comment_Blocks(t *testing.T) {
	tmpl, err := Compile(`<%# editor metadata lives here %><%= title %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"title": "Ready",
	}))
	require.NoError(t, err)
	require.Equal(t, "Ready", out)
}

func Test_VM_Fast_Render_Assignment_Scalar_Expressions_And_Index_Targets(t *testing.T) {
	tmpl, err := Compile(`<% let count = 0 %><% let label = "" %><% let data = {} %><%= for (_, item) in items { %><% count = count + 1 %><% label = label + item.Key %><% data[item.Key] = item.Value %><% } %><%= data[active] %>|<%= count %>|<%= label %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"active": "second",
		"items": []fastScriptPlanPair{
			{Key: "first", Value: "A"},
			{Key: "second", Value: "B"},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "B|2|firstsecond", out)
}

func Test_VM_Fast_Render_Receiver_Call_With_Arguments(t *testing.T) {
	tmpl, err := Compile(`<%= builder.RenderControl({name: "Email", type: inputType}) %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"builder":   fastScriptPlanBuilder{},
		"inputType": "text",
	}))
	require.NoError(t, err)
	require.Equal(t, "Email:text", out)
}

func Test_VM_Fast_Render_Receiver_Call_With_Scalar_And_Hash_Arguments(t *testing.T) {
	tmpl, err := Compile(`<%= builder.RenderSelect("Level", {options: ["one", "two"], value: selected}) %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"builder":  fastScriptPlanBuilder{},
		"selected": "two",
	}))
	require.NoError(t, err)
	require.Equal(t, "Level:2:two", out)
}

func Test_VM_Fast_Render_FormFor_Doc_Syntax_With_Helper_Block_Context(t *testing.T) {
	tmpl, err := Compile(`<%= form_for(record, {action: recordPath({id: record.ID}), method: "PUT"}) { %><%= f.InputTag("Title") %>|<%= f.SelectTag("Level", {options: ["one", "two"], value: selected}) %>|<%= f.CheckboxTag("Enabled", {unchecked: false}) %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"record":   fastScriptPlanRecord{ID: 7},
		"selected": "two",
		"recordPath": func(data map[string]interface{}) string {
			return fmt.Sprintf("/records/%v", data["id"])
		},
		"form_for": func(_ fastScriptPlanRecord, data map[string]interface{}, help plush.HelperContext) (template.HTML, error) {
			child := help.New()
			child.Set("f", fastScriptPlanBuilder{})
			body, err := help.BlockWith(child)
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<form action="/records/7" method="PUT">input:Title|select:Level:2:two|check:Enabled:false</form>`, out)
}

func Test_VM_Fast_Render_Form_Helper_Context_Set_Survives_Option_Map_Script(t *testing.T) {
	tmpl, err := Compile(`<%= form({action: submitPath(), method: "POST"}) { %><%= f.InputTag({name:"Fields[0].Value", value: row.Value, class:"field_input"}) %><% let options = {}; let selected = ""; if (row.Code == "") { selected = choices[0].ID } else { selected = row.Code }; for (candidate) in choices { options[candidate.Label] = candidate.ID } %><%= f.SelectTag({name:"Fields[0].Code", value: selected, options: options}) %><%= f.InputTag({name:"Fields[0].RecordID", value: record.ID, type:"hidden"}) %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"row": struct {
			Value int
			Code  string
		}{Value: 1},
		"record": fastScriptPlanRecord{ID: 7},
		"choices": []fastScriptPlanChoice{
			{ID: "first-id", Label: "First"},
			{ID: "second-id", Label: "Second"},
		},
		"submitPath": func() string {
			return "/records"
		},
		"form": func(data map[string]interface{}, help plush.HelperContext) (template.HTML, error) {
			help.Set("f", fastScriptPlanFormBuilder{})
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Contains(t, out, "input:Fields[0].Value:1:field_input")
	require.Contains(t, out, "select:Fields[0].Code:first-id:")
	require.Contains(t, out, "input:Fields[0].RecordID:7:&lt;nil&gt;")
}

func Test_VM_Fast_Render_Hctx_Block_Helper_Set_Is_Visible_To_Receiver_Calls(t *testing.T) {
	tmpl, err := Compile(`<%= if(record.Enabled) { %><%= form({action: submitPath(), method: "POST"}) { %><% let selected = record.Entries[0] %><%= if(input["target_id"] != "" && count(record.Entries) > 0) { %><%= for (_, candidate) in record.Entries { %><%= if(candidate.ID == input["target_id"]) { %><% selected = candidate %><% } %><% } %><% } %><%= f.InputTag({name:"Fields[0].EntryID", value: selected.ID, class:"field_input"}) %><% } %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	type entry struct {
		ID string
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"record": struct {
			Enabled bool
			Entries []entry
		}{
			Enabled: true,
			Entries: []entry{
				{ID: "first"},
				{ID: "second"},
			},
		},
		"input": map[string]interface{}{"target_id": "second"},
		"count": func(values []entry) int {
			return len(values)
		},
		"submitPath": func() string {
			return "/records"
		},
		"form": func(data tags.Options, help hctx.HelperContext) (template.HTML, error) {
			help.Set("f", fastScriptPlanFormBuilder{})
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<form action="/records" method="POST">input:Fields[0].EntryID:second:field_input</form>`, out)
}

func Test_VM_Fast_Render_Tags_Form_And_InputTag_Use_Direct_Calls(t *testing.T) {
	tmpl, err := Compile(`<%= form({method: "POST"}) { %><%= f.InputTag({name:"Name", value:"Mido"}) %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	ctx := plush.NewContextWith(map[string]interface{}{
		"form": func(_ tags.Options, help hctx.HelperContext) (template.HTML, error) {
			help.Set("f", fastScriptPlanTagFormBuilder{})
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML("<form>" + body + "</form>"), nil
		},
	})
	plush.EnableRenderVMHotspotDiagnostics(ctx)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<form><input name="Name" type="text" value="Mido" /></form>`, out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, 2, diagnostics.VMHotspots.HelperDirectCalls)
	require.Zero(t, diagnostics.VMHotspots.HelperReflectionCalls)

	paths := map[string]plush.RenderVMHelperCallPathDiagnostics{}
	for _, detail := range diagnostics.VMHotspots.HelperCallPaths {
		paths[detail.Name] = detail
	}
	require.Equal(t, plush.RenderVMHelperCallDirect, paths["form"].Path)
	require.Equal(t, plush.RenderVMHelperCallDirect, paths["InputTag"].Path)
}

func Test_VM_Fast_Render_Hctx_Block_Helper_Set_Survives_Reused_Bytecode_With_New_Context(t *testing.T) {
	tmpl, err := Compile(`<%= form({action: submitPath(), method: "POST"}) { %><%= f.InputTag({name:"Fields[0].Value", value: record.Value, class:"field_input"}) %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	type record struct {
		Value int
	}
	newCtx := func(value int, extraKey string) *plush.Context {
		ctx := plush.NewContextWith(map[string]interface{}{
			extraKey: "extra",
			"record": record{
				Value: value,
			},
			"submitPath": func() string {
				return "/records"
			},
			"form": func(data tags.Options, help hctx.HelperContext) (template.HTML, error) {
				help.Set("f", fastScriptPlanFormBuilder{})
				body, err := help.Block()
				if err != nil {
					return "", err
				}
				return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
			},
		})
		return ctx
	}

	first, err := tmpl.Render(newCtx(1, "first_extra"))
	require.NoError(t, err)
	require.Equal(t, `<form action="/records" method="POST">input:Fields[0].Value:1:field_input</form>`, first)

	second, err := tmpl.Render(newCtx(2, "second_extra"))
	require.NoError(t, err)
	require.Equal(t, `<form action="/records" method="POST">input:Fields[0].Value:2:field_input</form>`, second)
}

func Test_VM_Fast_Render_Hctx_Block_Helper_Set_Remains_Visible_After_Block_Call(t *testing.T) {
	tmpl, err := Compile(`<%= form({action: submitPath(), method: "POST"}) { %>inside<% } %>|<%= f.InputTag({name:"Fields[0].Value", value: record.Value, class:"field_input"}) %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"record": struct {
			Value int
		}{Value: 7},
		"submitPath": func() string {
			return "/records"
		},
		"form": func(data tags.Options, help hctx.HelperContext) (template.HTML, error) {
			help.Set("f", fastScriptPlanFormBuilder{})
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<form action="/records" method="POST">inside</form>|input:Fields[0].Value:7:field_input`, out)
}

func Test_VM_Fast_Render_Hctx_Block_Helper_Set_Is_Visible_Inside_Loop_Block_Call(t *testing.T) {
	tmpl, err := Compile(`<%= for (_, record) in records { %><%= form({action: submitPath(record.ID), method: "POST"}) { %><%= f.InputTag({name:"Fields[0].Value", value: record.Value, class:"field_input"}) %><% } %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	type record struct {
		ID    string
		Value int
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"records": []record{
			{ID: "first", Value: 1},
			{ID: "second", Value: 2},
		},
		"submitPath": func(id string) string {
			return "/records/" + id
		},
		"form": func(data tags.Options, help hctx.HelperContext) (template.HTML, error) {
			help.Set("f", fastScriptPlanFormBuilder{})
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<form action="/records/first" method="POST">input:Fields[0].Value:1:field_input</form><form action="/records/second" method="POST">input:Fields[0].Value:2:field_input</form>`, out)
}

func Test_VM_Fast_Render_Hctx_Block_Helper_Set_Is_Visible_To_Repeated_Receiver_Calls(t *testing.T) {
	tmpl, err := Compile(`<%= form({action: submitPath(), method: "POST", id: "control_set"}) { %>
<%= f.InputTag({name:"entity_id", value: record.ID}) %>
<%= f.InputTag({type:"text", name: "priority", value: 1}) %>
<%= f.InputTag({type:"text", name: "heading", value: "HEADING"}) %>
<%= f.InputTag({type:"text", name:"notes", value: "NOTES"}) %>
<%= f.InputTag({type:"text", name:"label", value: "sample"}) %>
<button role="submit">Save</button>
<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"record": fastScriptPlanRecord{ID: 7},
		"submitPath": func() string {
			return "/records"
		},
		"form": func(data tags.Options, help hctx.HelperContext) (template.HTML, error) {
			help.Set("f", fastScriptPlanFormBuilder{})
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Contains(t, out, "input:entity_id:7:&lt;nil&gt;")
	require.Contains(t, out, "input:priority:1:&lt;nil&gt;")
	require.Contains(t, out, "input:heading:HEADING:&lt;nil&gt;")
	require.Contains(t, out, "input:notes:NOTES:&lt;nil&gt;")
	require.Contains(t, out, "input:label:sample:&lt;nil&gt;")
}

func Test_VM_Fast_Render_Hctx_Block_Helper_Set_Is_Visible_To_Repeated_Tag_Calls(t *testing.T) {
	const input = `<%= form({action: submitPath(), method: "POST", id: "control_set"}) { %>
<%= f.InputTag({name:"entity_id", value: record.ID}) %>
<%= f.InputTag({type:"text", name: "priority", value: 1}) %>
<%= f.InputTag({type:"text", name: "heading", value: "HEADING"}) %>
<%= f.InputTag({type:"text", name:"notes", value: "NOTES"}) %>
<%= f.InputTag({type:"text", name:"label", value: "sample"}) %>
<button role="submit">Save</button>
<% } %>`
	tmpl, err := Compile(input)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := fastScriptPlanRepeatedTagFormContext()

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Contains(t, out, `<input name="entity_id" type="text" value="7" />`)
	require.Contains(t, out, `<input name="priority" type="text" value="1" />`)
	require.Contains(t, out, `<input name="heading" type="text" value="HEADING" />`)
	require.Contains(t, out, `<input name="notes" type="text" value="NOTES" />`)
	require.Contains(t, out, `<input name="label" type="text" value="sample" />`)
}

func Test_VM_Fast_Render_Hctx_Block_Helper_Set_Survives_Loop_Let_And_Break_Inside_Block(t *testing.T) {
	const input = `<%= form({action: submitPath(), method: "POST", id: "form_editor"}) { %>
<%= f.InputTag({name:"record_id", value: record.ID, type:"hidden"}) %>
<%= for (i, entry) in record.Entries { %>
	<%= if (i == 1) { break } %>
	<% let is_active = "" %>
	<%= if (input["entry_id"] != "" && entry.ID == input["entry_id"]) { %>
		<% is_active = "active" %>
	<% } %>
	<span class="<%= is_active %>"><%= entry.ID %></span>
<% } %>
<%= if(record.Enabled) { %>
	<%= f.InputTag({name:"count", value: 1, class:"child_count_input"}) %>
<% } %>
<% } %>`
	tmpl, err := Compile(input)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	type entry struct {
		ID string
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"record": struct {
			ID      int
			Enabled bool
			Entries []entry
		}{
			ID:      7,
			Enabled: true,
			Entries: []entry{{ID: "first"}, {ID: "second"}, {ID: "third"}},
		},
		"input": map[string]interface{}{"entry_id": "first"},
		"submitPath": func() string {
			return "/records"
		},
		"form": func(data tags.Options, help hctx.HelperContext) (template.HTML, error) {
			help.Set("f", fastScriptPlanTagFormBuilder{})
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Contains(t, out, `<form action="/records" method="POST">`)
	require.Contains(t, out, `<input name="record_id" type="hidden" value="7" />`)
	require.Contains(t, out, `<span class="active">first</span>`)
	require.NotContains(t, out, `third`)
	require.Contains(t, out, `<input class="child_count_input" name="count" type="text" value="1" />`)
}

func Test_VM_Fast_Render_Hctx_Block_Helper_Set_Survives_Long_Form_With_Break_And_Later_Let_Loop(t *testing.T) {
	const input = `<%= form({action: submitPath(), method: "POST", id: "form_editor"}) { %>
<%= f.SelectTag({name: "record_entry_id", value: selected, options: entryChoices, type:"hidden"}) %>
<%= f.InputTag({name:"record_id", value: record.ID, type:"hidden"}) %>
<%= for (index, entry) in record.Entries { %>
	<%= if (index == 30) { break } %>
	<%= if (entry.Asset) { %>
		<span><%= entry.Asset.URL %></span>
	<% } else { %>
		<%= for (_, asset) in record.Assets { %>
			<span><%= asset.URL %></span>
		<% } %>
	<% } %>
<% } %>
<%= for (fieldIndex, fieldName) in record.Fields { %>
	<%= if (fieldName == "secondary") { %>
		<%= for (i, entry) in record.Entries { %>
			<% let is_active = "" %>
			<%= if (input["entry_id"] != "" && entry.ID == input["entry_id"]) { %>
				<% is_active = "active" %>
			<% } %>
			<div class="<%= is_active %>"><%= entry.FieldValues[fieldIndex] %></div>
		<% } %>
	<% } %>
<% } %>
<%= if(record.Enabled) { %>
	<%= f.InputTag({name:"count", value: 1, class:"child_count_input"}) %>
<% } %>
<% } %>`
	tmpl, err := Compile(input)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	type asset struct {
		URL string
	}
	type entry struct {
		ID          string
		Asset       *asset
		FieldValues []string
	}
	entries := make([]entry, 35)
	for i := range entries {
		entries[i] = entry{
			ID:          fmt.Sprintf("entry-%d", i),
			FieldValues: []string{"primary", fmt.Sprintf("secondary-%d", i)},
		}
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"record": struct {
			ID      int
			Enabled bool
			Entries []entry
			Assets  []asset
			Fields  []string
		}{
			ID:      7,
			Enabled: true,
			Entries: entries,
			Assets:  []asset{{URL: "fallback.jpg"}},
			Fields:  []string{"primary", "secondary"},
		},
		"selected":     "entry-0",
		"entryChoices": map[string]interface{}{"primary secondary-0": "entry-0"},
		"input":        map[string]interface{}{"entry_id": "entry-2"},
		"submitPath": func() string {
			return "/records"
		},
		"form": func(data tags.Options, help hctx.HelperContext) (template.HTML, error) {
			help.Set("f", fastScriptPlanTagFormBuilder{})
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Contains(t, out, `<form action="/records" method="POST">`)
	require.Contains(t, out, `<select name="record_entry_id">`)
	require.Contains(t, out, `<input name="record_id" type="hidden" value="7" />`)
	require.Contains(t, out, `<div class="active">secondary-2</div>`)
	require.NotContains(t, out, `secondary-34</span>`)
	require.Contains(t, out, `<input class="child_count_input" name="count" type="text" value="1" />`)
}

func Test_VM_Fast_Render_Hctx_BlockWith_Helper_Set_Survives_Long_Form_With_Break_And_Later_Let_Loop(t *testing.T) {
	const input = `<%= form({action: submitPath(), method: "POST", id: "form_editor"}) { %>
<%= f.SelectTag({name: "record_entry_id", value: selected, options: entryChoices, type:"hidden"}) %>
<%= f.InputTag({name:"record_id", value: record.ID, type:"hidden"}) %>
<%= for (index, entry) in record.Entries { %>
	<%= if (index == 30) { break } %>
	<%= if (entry.Asset) { %>
		<span><%= entry.Asset.URL %></span>
	<% } else { %>
		<%= for (_, asset) in record.Assets { %>
			<span><%= asset.URL %></span>
		<% } %>
	<% } %>
<% } %>
<%= for (fieldIndex, fieldName) in record.Fields { %>
	<%= if (fieldName == "secondary") { %>
		<%= for (i, entry) in record.Entries { %>
			<% let is_active = "" %>
			<%= if (input["entry_id"] != "" && entry.ID == input["entry_id"]) { %>
				<% is_active = "active" %>
			<% } %>
			<div class="<%= is_active %>"><%= entry.FieldValues[fieldIndex] %></div>
		<% } %>
	<% } %>
<% } %>
<%= if(record.Enabled) { %>
	<%= f.InputTag({name:"count", value: 1, class:"child_count_input"}) %>
<% } %>
<% } %>`
	tmpl, err := Compile(input)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	type asset struct {
		URL string
	}
	type entry struct {
		ID          string
		Asset       *asset
		FieldValues []string
	}
	entries := make([]entry, 35)
	for i := range entries {
		entries[i] = entry{
			ID:          fmt.Sprintf("entry-%d", i),
			FieldValues: []string{"primary", fmt.Sprintf("secondary-%d", i)},
		}
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"record": struct {
			ID      int
			Enabled bool
			Entries []entry
			Assets  []asset
			Fields  []string
		}{
			ID:      7,
			Enabled: true,
			Entries: entries,
			Assets:  []asset{{URL: "fallback.jpg"}},
			Fields:  []string{"primary", "secondary"},
		},
		"selected":     "entry-0",
		"entryChoices": map[string]interface{}{"primary secondary-0": "entry-0"},
		"input":        map[string]interface{}{"entry_id": "entry-2"},
		"submitPath": func() string {
			return "/records"
		},
		"form": func(data tags.Options, help hctx.HelperContext) (template.HTML, error) {
			child := help.New()
			child.Set("f", fastScriptPlanTagFormBuilder{})
			body, err := help.BlockWith(child)
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Contains(t, out, `<form action="/records" method="POST">`)
	require.Contains(t, out, `<select name="record_entry_id">`)
	require.Contains(t, out, `<input name="record_id" type="hidden" value="7" />`)
	require.Contains(t, out, `<div class="active">secondary-2</div>`)
	require.NotContains(t, out, `secondary-34</span>`)
	require.Contains(t, out, `<input class="child_count_input" name="count" type="text" value="1" />`)
}

func Test_VM_Bytecode_Render_Hctx_Block_Helper_Set_Is_Visible_To_Repeated_Tag_Calls(t *testing.T) {
	const input = `<%= form({action: submitPath(), method: "POST", id: "control_set"}) { %>
<%= f.InputTag({name:"entity_id", value: record.ID}) %>
<%= f.InputTag({type:"text", name: "priority", value: 1}) %>
<%= f.InputTag({type:"text", name: "heading", value: "HEADING"}) %>
<%= f.InputTag({type:"text", name:"notes", value: "NOTES"}) %>
<%= f.InputTag({type:"text", name:"label", value: "sample"}) %>
<button role="submit">Save</button>
<% } %>`
	tmpl, err := Compile(input)
	require.NoError(t, err)

	out, err := renderBytecodeVMWithState(tmpl.bytecode, fastScriptPlanRepeatedTagFormContext(), "", false, "")
	require.NoError(t, err)
	require.Contains(t, out, `<input name="entity_id" type="text" value="7" />`)
	require.Contains(t, out, `<input name="priority" type="text" value="1" />`)
	require.Contains(t, out, `<input name="heading" type="text" value="HEADING" />`)
	require.Contains(t, out, `<input name="notes" type="text" value="NOTES" />`)
	require.Contains(t, out, `<input name="label" type="text" value="sample" />`)
}

func fastScriptPlanRepeatedTagFormContext() *plush.Context {
	return plush.NewContextWith(map[string]interface{}{
		"record": fastScriptPlanRecord{ID: 7},
		"submitPath": func() string {
			return "/records"
		},
		"form": func(data tags.Options, help hctx.HelperContext) (template.HTML, error) {
			builder := fastScriptPlanTagFormBuilder{}
			help.Set("f", builder)
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
		},
	})
}

func Test_VM_Fast_Render_Hctx_Block_Helper_Set_Is_Visible_After_Earlier_Form_And_Shadowing_Loop(t *testing.T) {
	tmpl, err := Compile(`<%= if (record) { %>
<%= form({action: firstPath(), method: "POST"}) { %>
<%= f.InputTag({name:"first_id", value: record.ID}) %>
<% } %>
<%= if (related && related.Items) { %>
<%= for (record) in related.Items { %>
<%= form({action: itemPath(record.ID), method: "POST"}) { %>
<%= f.InputTag({name:"item_id", value: record.ID}) %>
<% } %>
<% } %>
<% } %>
<%= form({action: submitPath(), method: "POST", id: "control_set"}) { %>
<%= f.InputTag({name:"entity_id", value: record.ID}) %>
<%= f.InputTag({type:"text", name: "priority", value: 1}) %>
<%= f.InputTag({type:"text", name: "heading", value: "HEADING"}) %>
<%= f.InputTag({type:"text", name:"notes", value: "NOTES"}) %>
<%= f.InputTag({type:"text", name:"label", value: "sample"}) %>
<% } %>
<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	type record struct {
		ID int
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"record": record{ID: 7},
		"related": struct {
			Items []record
		}{Items: []record{{ID: 11}}},
		"firstPath": func() string {
			return "/first"
		},
		"itemPath": func(id int) string {
			return fmt.Sprintf("/items/%d", id)
		},
		"submitPath": func() string {
			return "/records"
		},
		"form": func(data tags.Options, help hctx.HelperContext) (template.HTML, error) {
			help.Set("f", fastScriptPlanTagFormBuilder{})
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Contains(t, out, `<form action="/first" method="POST">`)
	require.Contains(t, out, `<form action="/items/11" method="POST">`)
	require.Contains(t, out, `<form action="/records" method="POST">`)
	require.Contains(t, out, `<input name="entity_id" type="text" value="7" />`)
	require.Contains(t, out, `<input name="notes" type="text" value="NOTES" />`)
}

func Test_VM_Fast_Render_Form_Helper_Context_Set_Is_Visible_To_Nested_Render_Helper(t *testing.T) {
	tmpl, err := Compile(`<%= form({action: submitPath(), method: "POST"}) { %><%= for (_, block) in blocks { %><%= if(!block.Hidden) { %><%= render(block.Type + ".plush.html", {settings: block}) %><% } %><% } %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"blocks": []struct {
			Type   string
			Hidden bool
		}{{Type: "panel", Hidden: false}},
		"submitPath": func() string {
			return "/records"
		},
		"form": func(data map[string]interface{}, help plush.HelperContext) (template.HTML, error) {
			help.Set("f", fastScriptPlanFormBuilder{})
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" method="%s">%s</form>`, data["action"], data["method"], body)), nil
		},
		"render": func(string, map[string]interface{}, plush.HelperContext) (template.HTML, error) {
			return "", nil
		},
	})
	SetFastHelper(ctx, "render", func(w FastWriter, args FastArgs) error {
		fileName, ok := args.String(0)
		if !ok || fileName != "panel.plush.html" {
			return ErrFastUnsupported
		}
		data, ok := args.Raw(1)
		if !ok {
			return ErrFastUnsupported
		}
		values, ok := data.(map[string]interface{})
		if !ok {
			return ErrFastUnsupported
		}
		help := plush.NewHelperContext(w.Context(), nil)
		child := help.New()
		for k, v := range values {
			child.Set(k, v)
		}
		rendered, err := plush.Render(`<%= f.InputTag({name:"Fields[0].Value", value: 1, class:"field_input"}) %>`, child)
		if err != nil {
			return err
		}
		w.WriteHTMLString(rendered)
		return nil
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<form action="/records" method="POST">input:Fields[0].Value:1:field_input</form>`, out)
}

func Test_VM_Fast_Render_Loop_Partial_Data_Can_Use_Current_Value(t *testing.T) {
	tmpl, err := Compile(`<%= for (_, entry) in entries { %><%= partial("entry-card.plush", {entry: entry}) %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"entries": []fastScriptPlanChoice{
			{ID: "first", Label: "First"},
			{ID: "second", Label: "Second"},
		},
		"partialFeeder": func(name string) (string, error) {
			require.Equal(t, "entry-card.plush", name)
			return `<span><%= entry.Label %></span>`, nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<span>First</span><span>Second</span>`, out)
}

func Test_VM_Fast_Render_Nested_Collection_Loop_Reads_Value_Field(t *testing.T) {
	for name, entries := range map[string]interface{}{
		"value slice":   fastScriptPlanAssetSet{Assets: []fastScriptPlanAsset{{URL: "/first.png"}, {URL: "/second.png"}}},
		"pointer slice": struct{ Assets []*fastScriptPlanAsset }{Assets: []*fastScriptPlanAsset{{URL: "/first.png"}, {URL: "/second.png"}}},
	} {
		t.Run(name, func(t *testing.T) {
			tmpl, err := Compile(`<%= for (index, asset) in set.Assets { %><link data-index="<%= index %>" href="<%= asset.URL %>" /><% } %>`)
			require.NoError(t, err)
			require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
			require.Empty(t, tmpl.bytecode.FastReject)

			out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
				"set": entries,
			}))
			require.NoError(t, err)
			require.Equal(t, `<link data-index="0" href="/first.png" /><link data-index="1" href="/second.png" />`, out)
		})
	}
}

func Test_VM_Fast_Render_Silent_Block_Helper_Discards_Return_And_Renders_Block(t *testing.T) {
	tmpl, err := Compile(`<% capture("extra") { %><style><%= klass %></style><% } %><%= readCapture("extra") %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	require.True(t, tmpl.bytecode.FastRenderPlan.Segments[0].BlockCall.Silent)

	ctx := plush.NewContextWith(map[string]interface{}{
		"klass": "online",
		"capture": func(name string, help plush.HelperContext) (template.HTML, error) {
			rendered, err := help.Block()
			if err != nil {
				return "", err
			}
			help.Set("capture:"+name, rendered)
			return template.HTML("visible return should be discarded"), nil
		},
		"readCapture": func(name string, help plush.HelperContext) template.HTML {
			value, _ := help.Value("capture:" + name).(string)
			return template.HTML(value)
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "<style>online</style>", out)
}

func Test_VM_Fast_Render_Silent_Script_For_Discards_Output_And_Assigns(t *testing.T) {
	tmpl, err := Compile(`<% let collected = [] %><% for (_, item) in items { %>ignored<%= item.Value %><% collected = collected + item.Value %><% } %><%= collected[1] %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	require.True(t, tmpl.bytecode.FastRenderPlan.Segments[1].Loop.Silent)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"items": []struct {
			Value string
		}{
			{Value: "first"},
			{Value: "second"},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "second", out)
}

func Test_VM_Fast_Render_Loop_Branch_Appends_Local_Value_To_Outer_Array(t *testing.T) {
	tmpl, err := Compile(`<% let handlers = [] %><% let items = [1,2,3] %><%= for (index, item) in items { %><%= if(true) { %><% let handle = "test" + to_string(index) %><% handlers = handlers + handle %><% } %><% } %><%= handlers[0] %>|<%= handlers[1] %>|<%= handlers[2] %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"to_string": strconv.Itoa,
	}))
	require.NoError(t, err)
	require.Equal(t, "test0|test1|test2", out)
}

func Test_VM_Fast_Render_Branch_Appends_Local_Value_To_Outer_Array(t *testing.T) {
	tmpl, err := Compile(`<% let handlers = [] %><%= if(true) { %><% let handle = "test0" %><% handlers = handlers + handle %><% } %><%= handlers[0] %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	out, err := tmpl.Render(plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "test0", out)
}

func Test_VM_Fast_Render_Scoped_Assignment_Path_Matrix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "output conditional else branch",
			input:    `<% let result = "" %><%= if (false) { %><% let local = "wrong" %><% result = local %><% } else { %><% let local = "A" %><% result = local %><% } %><%= result %>`,
			expected: "A",
		},
		{
			name:     "silent conditional branch",
			input:    `<% let result = "" %><% if (true) { %>discarded<% let local = "A" %><% result = local %><% } %><%= result %>`,
			expected: "A",
		},
		{
			name:     "output loop conditional branch",
			input:    `<% let result = "" %><%= for (_, item) in ["A","B"] { %><%= if (true) { %><% let local = item %><% result = result + local %><% } %><% } %><%= result %>`,
			expected: "AB",
		},
		{
			name:     "silent loop conditional branch",
			input:    `<% let result = "" %><%= for (_, item) in ["A","B"] { %><% if (true) { %>discarded<% let local = item %><% result = result + local %><% } %><% } %><%= result %>`,
			expected: "AB",
		},
		{
			name:     "nested loop assignment",
			input:    `<% let result = "" %><%= for (_, row) in [["A","B"],["C"]] { %><%= for (_, item) in row { %><% let local = item %><% result = result + local %><% } %><% } %><%= result %>`,
			expected: "ABC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := Compile(tt.input)
			require.NoError(t, err)
			require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
			require.Empty(t, tmpl.bytecode.FastReject)

			ctx := plush.NewContext()
			out, err := tmpl.Render(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, out)

			diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
			require.True(t, ok)
			require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
		})
	}
}

func Test_VM_Fast_Render_Recursive_Loop_Partial_Syncs_Parent_Assignment(t *testing.T) {
	partialSource := `<%= for (_, node) in nodes { %>` +
		`<%= if (node.Enabled) { %>` +
		`<% let entry = "enter:" + node.Label + "|" %>` +
		`<% collected = collected + entry %>` +
		`<%= if (len(node.Children) > 0) { %>` +
		`<%= partial("tree.plush.html", {nodes: node.Children}) %>` +
		`<% } %>` +
		`<% let exit = "exit:" + node.Label + "|" %>` +
		`<% collected = collected + exit %>` +
		`<% } %>` +
		`<% } %>`

	partialTemplate, err := Compile(partialSource)
	require.NoError(t, err)
	require.NotNil(t, partialTemplate.bytecode.FastRenderPlan, partialTemplate.bytecode.FastReject)
	require.Empty(t, partialTemplate.bytecode.FastReject)

	rootTemplate, err := Compile(`<% let collected = "" %><%= partial("tree.plush.html", {nodes: nodes}) %><%= collected %>`)
	require.NoError(t, err)
	require.NotNil(t, rootTemplate.bytecode.FastRenderPlan, rootTemplate.bytecode.FastReject)
	require.Empty(t, rootTemplate.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"nodes": []fastScriptPlanRecursiveNode{
			{
				Label:   "root",
				Enabled: true,
				Children: []fastScriptPlanRecursiveNode{
					{
						Label:   "branch",
						Enabled: true,
						Children: []fastScriptPlanRecursiveNode{
							{
								Label:   "leaf",
								Enabled: true,
								Children: []fastScriptPlanRecursiveNode{
									{Label: "tip", Enabled: true},
								},
							},
						},
					},
				},
			},
		},
		"partialFeeder": func(name string) (string, error) {
			if name != "tree.plush.html" {
				return "", fmt.Errorf("unexpected partial %q", name)
			}
			return partialSource, nil
		},
	})

	out, err := rootTemplate.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "enter:root|enter:branch|enter:leaf|enter:tip|exit:tip|exit:leaf|exit:branch|exit:root|", out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.NotEqual(t, plush.RenderFastPathInterpreterFallback, diagnostics.FastPath)
}

func Test_VM_Fast_Render_Block_Helper_Allows_Local_Assignment_Setup(t *testing.T) {
	tmpl, err := Compile(`<%= wrap({}) { %><% let options = {} %><% for (_, item) in items { %><% options[item.Key] = item.Value %><% } %><%= builder.RenderControl({name: options[active], type: "hidden"}) %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan.Segments[0].BlockCall.BlockBytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"active":  "second",
		"builder": fastScriptPlanBuilder{},
		"items": []fastScriptPlanPair{
			{Key: "first", Value: "A"},
			{Key: "second", Value: "B"},
		},
		"wrap": func(_ map[string]interface{}, help plush.HelperContext) (template.HTML, error) {
			rendered, err := help.Block()
			return template.HTML("[" + rendered + "]"), err
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "[B:hidden]", out)
}

func Test_VM_Fast_Render_Loop_Local_Assignment_Does_Not_Leak(t *testing.T) {
	tmpl, err := Compile(`<% let out = "" %><%= for (_, item) in items { %><% let label = "" %><% label = item %><% out = out + label %><% } %><%= out %><%= if (label) { %>leak<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"items": []string{"a", "b"},
	}))
	require.NoError(t, err)
	require.Equal(t, "ab", out)
}

func Test_VM_Fast_Render_Branch_Local_Shadow_Does_Not_Replace_Outer_Assignment(t *testing.T) {
	tmpl, err := Compile(`<% let result = "" %><%= for (_, item) in items { %><% result = result + "A" %><%= if (true) { %><% let result = "inner" %><%= result %><% } %><% result = result + "B" %><% } %>|<%= result %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"items": []int{1, 2},
	}))
	require.NoError(t, err)
	require.Equal(t, "innerinner|ABAB", out)
}

func Test_VM_Fast_Render_Assignment_Before_Same_Scope_Shadow_Matches_Interpreter(t *testing.T) {
	source := `<% let result = "" %><%= for (_, item) in items { %><% result = result + "A" %><% let result = "inner" %><% result = result + "X" %><% } %><%= result %>`
	data := map[string]interface{}{"items": []int{1}}

	expected, err := plush.RenderInterpreter(source, plush.NewContextWith(data))
	require.NoError(t, err)

	tmpl, err := Compile(source)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	out, err := tmpl.Render(plush.NewContextWith(data))
	require.NoError(t, err)
	require.Equal(t, expected, out)
}

func Test_VM_Fast_Render_Binding_Metadata_Budget_Fallback_Matches_Interpreter(t *testing.T) {
	const depth = 64

	var source strings.Builder
	for index := 0; index < depth; index++ {
		source.WriteString(`<% let value`)
		source.WriteString(strconv.Itoa(index))
		source.WriteString(` = "" %>`)
	}
	for index := 0; index < depth; index++ {
		source.WriteString(`<%= if (true) { %><% value`)
		source.WriteString(strconv.Itoa(index))
		source.WriteString(` = "updated" %>`)
	}
	for index := 0; index < depth; index++ {
		source.WriteString(`<% } %>`)
	}
	source.WriteString(`<%= value63 %>`)

	expected, err := plush.RenderInterpreter(source.String(), plush.NewContext())
	require.NoError(t, err)

	tmpl, err := Compile(source.String())
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	prepared, unprepared := fastBindingSyncConditionalPlanCounts(tmpl.bytecode.FastRenderPlan.Segments)
	require.Positive(t, prepared)
	require.Positive(t, unprepared)

	out, err := tmpl.Render(plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, expected, out)
}

func fastBindingSyncConditionalPlanCounts(segments []compiler.FastRenderSegment) (prepared, unprepared int) {
	for i := range segments {
		segment := &segments[i]
		if segment.Kind != compiler.FastRenderSegmentConditional || segment.Conditional == nil {
			continue
		}
		for branchIndex := range segment.Conditional.Branches {
			branch := &segment.Conditional.Branches[branchIndex]
			if branch.BindingSync.Prepared {
				prepared++
			} else {
				unprepared++
			}
			childPrepared, childUnprepared := fastBindingSyncConditionalPlanCounts(branch.Segments)
			prepared += childPrepared
			unprepared += childUnprepared
		}
		childPrepared, childUnprepared := fastBindingSyncConditionalPlanCounts(segment.Conditional.ElseSegments)
		prepared += childPrepared
		unprepared += childUnprepared
	}
	return prepared, unprepared
}

func Test_VM_Fast_Render_Prefix_Condition_And_Loop_Concat(t *testing.T) {
	tmpl, err := Compile(`<%= if (!userSignedIn) { %>Guest<% } else { %>User<% } %><%= for (item) in menu.Items { %><%= item.Name + " x " + item.Count %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"userSignedIn": false,
		"menu": fastScriptPlanMenu{Items: []fastScriptPlanItem{
			{Name: "One", Count: 2},
			{Name: "Two", Count: 3},
		}},
	}))
	require.NoError(t, err)
	require.Equal(t, `GuestOne x 2;Two x 3;`, out)
}

func Test_VM_Fast_Render_Nested_Loop_Uses_Outer_Key(t *testing.T) {
	tmpl, err := Compile(`<%= for (i, row) in rows { %><%= for (j, col) in row { %><%= i %>,<%= j %>:<%= col %>;<% } %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"rows": [][]string{{"a", "b"}, {"c"}},
	}))
	require.NoError(t, err)
	require.Equal(t, `0,0:a;0,1:b;1,0:c;`, out)
}

func Test_VM_Fast_Render_Nested_Loop_Condition_Uses_Outer_Value(t *testing.T) {
	tmpl, err := Compile(`<%= for (k, messages) in flash { %><%= for (msg) in messages { %><%= if (len(messages) && messages[0] != "skip") { %><%= k %>:<%= msg %>;<% } %><% } %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"flash": map[string][]string{"notice": {"Hello", "Bye"}},
	}))
	require.NoError(t, err)
	require.Equal(t, `notice:Hello;notice:Bye;`, out)
}

func Test_VM_Fast_Render_Silent_Script_If_Discards_Output_But_Evaluates_Selected_Branch(t *testing.T) {
	tmpl, err := Compile(`<% if (show) { %>hidden<%= touch(name) %><% } else { %><%= touch("else") %><% } %>done`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.True(t, tmpl.bytecode.FastRenderPlan.Segments[0].Conditional.Silent)

	calls := []string{}
	ctx := plush.NewContextWith(map[string]interface{}{
		"show": true,
		"name": "Mido",
		"touch": func(value string) string {
			calls = append(calls, value)
			return "called " + value
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "done", out)
	require.Equal(t, []string{"Mido"}, calls)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)

	calls = calls[:0]
	out, err = tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"show": false,
		"name": "Mido",
		"touch": func(value string) string {
			calls = append(calls, value)
			return "called " + value
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "done", out)
	require.Equal(t, []string{"else"}, calls)
}

func Test_VM_Fast_Render_Silent_Script_If_Inside_Loop(t *testing.T) {
	type item struct {
		Name   string
		Hidden bool
		Stop   bool
	}

	tmpl, err := Compile(`<%= for (_, item) in items { %><% if (item.Hidden) { %>hidden<%= touch(item.Name) %><% } %><% if (item.Stop) { %><%= break %><% } %><%= item.Name %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	calls := []string{}
	ctx := plush.NewContextWith(map[string]interface{}{
		"items": []item{
			{Name: "A"},
			{Name: "B", Hidden: true},
			{Name: "C", Stop: true},
			{Name: "D"},
		},
		"touch": func(value string) string {
			calls = append(calls, value)
			return value
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "A;B;", out)
	require.Equal(t, []string{"B"}, calls)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
}

func Test_VM_Fast_Render_Regex_And_NonString_Hash_Keys(t *testing.T) {
	tmpl, err := Compile(`<%= name ~= "^Mi" %>|<%= {true: "yes", 7: "lucky"}[true] %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	regexSlot := tmpl.bytecode.FastRenderPlan.Segments[0].ValuePlan.RegexCache
	require.NotNil(t, regexSlot)
	require.Nil(t, regexSlot.Load())

	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
	})
	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "true|yes", out)
	firstRegex := regexSlot.Load().(regexCacheEntry)
	require.Equal(t, "^Mi", firstRegex.pattern)
	require.NotNil(t, firstRegex.re)

	out, err = tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"name": "Mina",
	}))
	require.NoError(t, err)
	require.Equal(t, "true|yes", out)
	secondRegex := regexSlot.Load().(regexCacheEntry)
	require.Same(t, firstRegex.re, secondRegex.re)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
	require.Empty(t, diagnostics.FastReject)
}

func Test_VM_Fast_Render_Identifier_With_Digits_Unary_Minus(t *testing.T) {
	tmpl, err := Compile(`<%= -my123greet %>|<%= -my123greet + 10 %>|<%= -my123greet - 10 %>|<%= -my123greet + my123greet2 %>|<%= -my123int64 + my123float64 %>|<%= -my123int %>|<%= -my123count + 1 %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	require.Equal(t, []string{"my123greet", "my123greet2", "my123int64", "my123float64", "my123int", "my123count"}, tmpl.bytecode.FastRenderPlan.Bindings)
	require.Equal(t, compiler.FastValuePrefix, tmpl.bytecode.FastRenderPlan.Segments[0].ValuePlan.Kind)
	require.Equal(t, "-", tmpl.bytecode.FastRenderPlan.Segments[0].ValuePlan.Operator)

	ctx := plush.NewContextWith(map[string]interface{}{
		"my123greet":   float64(5.5),
		"my123greet2":  int64(-10),
		"my123int64":   int64(10),
		"my123float64": float64(5.5),
		"my123int":     int(4),
		"my123count":   int64(6),
	})
	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "-5.5|4.5|-15.5|-15.5|-4.5|-4|-5", out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
	require.Empty(t, diagnostics.FastReject)
}

func Test_VM_Fast_Render_Silent_Plain_Helper_Call_Discards_Return(t *testing.T) {
	tmpl, err := Compile(`<% touch(name) %><%= done %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	require.True(t, tmpl.bytecode.FastRenderPlan.Segments[0].Call.Silent)

	calls := []string{}
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"done": "ok",
		"touch": func(value string) string {
			calls = append(calls, value)
			return "hidden"
		},
	})
	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "ok", out)
	require.Equal(t, []string{"Mido"}, calls)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
}

func Test_VM_Fast_Render_Dynamic_Partial_Name_And_Layout(t *testing.T) {
	tmpl, err := Compile(`<%= partial(templateName, {name: name, layout: layoutName}) %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	require.Equal(t, compiler.FastRenderSegmentCall, tmpl.bytecode.FastRenderPlan.Segments[0].Kind)
	require.Equal(t, "partial", tmpl.bytecode.FastRenderPlan.Segments[0].Call.Name)

	ctx := plush.NewContextWith(map[string]interface{}{
		"templateName": "row.plush",
		"layoutName":   "shell.plush",
		"name":         "Mido",
		"partialFeeder": func(name string) (string, error) {
			switch name {
			case "row.plush":
				return `<span><%= name %></span>`, nil
			case "shell.plush":
				return `<section><%= yield %></section>`, nil
			default:
				return "", fmt.Errorf("missing partial %s", name)
			}
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "<section><span>Mido</span></section>", out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
}

func Test_VM_Fast_Render_Dynamic_Callee_And_Chained_Receiver_Call(t *testing.T) {
	tmpl, err := Compile(`<%= helpers[name]("x") %>|<%= makeUser(name).Render("short") %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "echo",
		"helpers": map[string]interface{}{
			"echo": func(value string) string {
				return "dyn:" + value
			},
		},
		"makeUser": func(name string) fastScriptPlanUser {
			return fastScriptPlanUser{Name: name}
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "dyn:x|short:echo", out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
}

func Test_VM_Fast_Render_Loop_Registered_Helper_Context_Includes_Loop_Binding(t *testing.T) {
	tmpl, err := Compile(`<%= for (_, block) in blocks { %><%= render(block.Type + ".plush.html", {settings: block}) %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"blocks": []fastScriptPlanRenderBlock{{Type: "panel-option", BlockID: "test-1234"}},
		"render": func(string, map[string]interface{}, plush.HelperContext) (template.HTML, error) {
			return "", nil
		},
	})
	SetFastHelper(ctx, "render", func(w FastWriter, args FastArgs) error {
		fileName, ok := args.String(0)
		if !ok {
			return ErrFastUnsupported
		}
		rawData, ok := args.Raw(1)
		if !ok {
			return ErrFastUnsupported
		}
		data, ok := rawData.(map[string]interface{})
		if !ok {
			return ErrFastUnsupported
		}
		settings, ok := data["settings"].(fastScriptPlanRenderBlock)
		if !ok {
			return fmt.Errorf("%q: unknown identifier", "settings")
		}
		block, ok := w.Context().Value("block").(fastScriptPlanRenderBlock)
		if !ok {
			return fmt.Errorf("%q: unknown identifier", "block")
		}
		w.WriteHTMLString(fileName + "|" + settings.BlockID + "|" + block.BlockID)
		return nil
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "panel-option.plush.html|test-1234|test-1234", out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
	require.Empty(t, diagnostics.FastReject)
}

func Test_VM_Fast_Render_Output_If_Return_Nil_Loop_And_Iterator_Assignment(t *testing.T) {
	tmpl, err := Compile(`<%= if active { return "yes" } %>after`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	ctx := plush.NewContextWith(map[string]interface{}{"active": true})
	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "yesafter", out)
	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)

	tmpl, err = Compile(`<%= for (_, item) in nil { %>x<% } %><%= for (_, item) in items { %><% item = "x" %><%= item %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	ctx = plush.NewContextWith(map[string]interface{}{
		"items": []string{"a", "b"},
	})
	out, err = tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "xx", out)

	tmpl, err = Compile(`<%= for (_, value) in items { %><% if (value == 1) { %>P<% return "A" } %><%= value %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	ctx = plush.NewContextWith(map[string]interface{}{
		"items": []int{1, 2},
	})
	out, err = tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "PA2", out)
}

func Test_VM_Fast_Render_Len_Nil_Collection_Short_Circuits_Index(t *testing.T) {
	for name, tt := range map[string]struct {
		record   interface{}
		template string
	}{
		"typed nil slice field": {
			record:   fastScriptPlanNodeSet{},
			template: `<%= if (len(record.Nodes) > 0 && record.Nodes[0].Label) { %>present<% } else { %>empty<% } %>`,
		},
		"nil map value": {
			record:   map[string]interface{}{"Nodes": nil},
			template: `<%= if (len(record.Nodes) > 0 && record.Nodes[0].Label) { %>present<% } else { %>empty<% } %>`,
		},
		"missing map value": {
			record:   map[string]interface{}{},
			template: `<%= if (len(record.Nodes) > 0 && record.Nodes[0].Label) { %>present<% } else { %>empty<% } %>`,
		},
		"assigned nil collection": {
			record:   fastScriptPlanNodeSet{},
			template: `<% let nodes = record.Nodes %><%= if (len(nodes) > 1) { %>present<% } else { %>empty<% } %>`,
		},
		"assigned nil map value": {
			record:   map[string]interface{}{"Nodes": nil},
			template: `<% let nodes = record.Nodes %><%= if (len(nodes) > 1) { %>present<% } else { %>empty<% } %>`,
		},
		"assigned missing map value": {
			record:   map[string]interface{}{},
			template: `<% let nodes = record.Nodes %><%= if (len(nodes) > 1) { %>present<% } else { %>empty<% } %>`,
		},
		"loop typed nil slice field": {
			record:   []fastScriptPlanNodeSet{{}},
			template: `<%= for (_, record) in record { %><%= if (len(record.Nodes) > 1) { %>present<% } else { %>empty<% } %><% } %>`,
		},
		"loop nil map value": {
			record:   []map[string]interface{}{{"Nodes": nil}},
			template: `<%= for (_, record) in record { %><%= if (len(record.Nodes) > 1) { %>present<% } else { %>empty<% } %><% } %>`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			tmpl, err := Compile(tt.template)
			require.NoError(t, err)
			require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
			require.Empty(t, tmpl.bytecode.FastReject)

			ctx := plush.NewContextWith(map[string]interface{}{
				"record": tt.record,
			})
			out, err := tmpl.Render(ctx)
			require.NoError(t, err)
			require.Equal(t, "empty", out)

			diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
			require.True(t, ok)
			require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
			require.Empty(t, diagnostics.FastReject)
		})
	}
}

func Test_VM_Fast_Render_Function_Literal_Uses_Generic_VM_Not_Interpreter_Fallback(t *testing.T) {
	tmpl, err := Compile(`<% let add = fn(x) { return x + 1 } %><%= add(2) %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContext()
	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "3", out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathGeneric, diagnostics.FastPath)
	require.NotEqual(t, plush.RenderFastPathInterpreterFallback, diagnostics.FastPath)
	require.Empty(t, diagnostics.FastReject)
}
