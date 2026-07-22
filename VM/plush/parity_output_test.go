package plush_test

import (
	"fmt"
	"html/template"
	"testing"
	"time"
)

type phase7HTMLer struct {
	value string
}

func (h phase7HTMLer) HTML() template.HTML {
	return template.HTML(h.value)
}

type phase7InterfaceValue struct {
	value interface{}
}

func (v phase7InterfaceValue) Interface() interface{} {
	return v.value
}

type phase7Stringer string

func (s phase7Stringer) String() string {
	return string(s)
}

func Test_Parity_Phase_7_Escaping_And_HTML_Safety(t *testing.T) {
	ctx := contextWith(map[string]interface{}{
		"unsafe":          `<strong>"x"</strong>`,
		"safeHTML":        template.HTML(`<strong>"x"</strong>`),
		"htmler":          phase7HTMLer{value: `<em>x</em>`},
		"interfaceHTML":   phase7InterfaceValue{value: template.HTML(`<i>x</i>`)},
		"interfaceString": phase7InterfaceValue{value: `<i>x</i>`},
		"rawStringer":     phase7Stringer(`<b>stringer</b>`),
	})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal string literal escapes",
			input:    `<%= "<script>alert('x')</script>" %>`,
			expected: `&lt;script&gt;alert(&#39;x&#39;)&lt;/script&gt;`,
		},
		{
			name:     "injected variable escapes",
			input:    `<%= unsafe %>`,
			expected: `&lt;strong&gt;&#34;x&#34;&lt;/strong&gt;`,
		},
		{
			name:     "template html remains raw",
			input:    `<%= safeHTML %>`,
			expected: `<strong>"x"</strong>`,
		},
		{
			name:     "htmler remains raw",
			input:    `<%= htmler %>`,
			expected: `<em>x</em>`,
		},
		{
			name:     "interface unwraps html",
			input:    `<%= interfaceHTML %>`,
			expected: `<i>x</i>`,
		},
		{
			name:     "interface unwraps escaped string",
			input:    `<%= interfaceString %>`,
			expected: `&lt;i&gt;x&lt;/i&gt;`,
		},
		{
			name:     "fmt stringer matches interpreter",
			input:    `<%= rawStringer %>`,
			expected: `<b>stringer</b>`,
		},
		{
			name:     "raw helper remains raw",
			input:    `<%= raw("<u>x</u>") %>`,
			expected: `<u>x</u>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireBothRender(t, tt.input, tt.expected, ctx)
		})
	}
}

func Test_Parity_Phase_7_Time_Formatting(t *testing.T) {
	tm := time.Date(2013, time.February, 3, 0, 0, 0, 0, time.UTC)

	requireBothRender(t, `<%= tm %>|<%= ptr %>`, "February 03, 2013 00:00:00 +0000|February 03, 2013 00:00:00 +0000", contextWith(map[string]interface{}{
		"tm":  tm,
		"ptr": &tm,
	}))

	requireBothRender(t, `<%= tm %>|<%= ptr %>`, "2013-03-Feb|2013-03-Feb", contextWith(map[string]interface{}{
		"tm":          tm,
		"ptr":         &tm,
		"TIME_FORMAT": "2006-02-Jan",
	}))
}

func Test_Parity_Phase_7_Numbers_Bools_Arrays_And_Nil(t *testing.T) {
	ctx := contextWith(map[string]interface{}{
		"i":       int32(1),
		"u":       uint32(2),
		"f":       float32(3.5),
		"truthy":  true,
		"falsy":   false,
		"strings": []string{"a", "<b>"},
		"mixed":   []interface{}{"x", template.HTML("<y>"), 7, false, nil},
	})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "numbers render like interpreter",
			input:    `<%= i %>|<%= u %>|<%= f %>|<%= 4 %>|<%= 5.25 %>`,
			expected: `1|2|3.5|4|5.25`,
		},
		{
			name:     "bools render like interpreter",
			input:    `<%= truthy %>|<%= falsy %>|<%= true %>|<%= false %>`,
			expected: `true|false|true|false`,
		},
		{
			name:     "array literal writes elements",
			input:    `<%= ["a", "<b>", 3, false, nil] %>`,
			expected: `a&lt;b&gt;3false`,
		},
		{
			name:     "native string slice writes elements",
			input:    `<%= strings %>`,
			expected: `a&lt;b&gt;`,
		},
		{
			name:     "native interface slice writes elements",
			input:    `<%= mixed %>`,
			expected: `x<y>7false`,
		},
		{
			name:     "nil renders empty",
			input:    `<%= nil %>`,
			expected: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireBothRender(t, tt.input, tt.expected, ctx)
		})
	}
}

func Test_Parity_Phase_7_Interface_Stringer_Ordering(t *testing.T) {
	value := phase7InterfaceValue{value: phase7Stringer(fmt.Sprintf("<%s>", "wrapped"))}
	requireBothRender(t, `<%= value %>`, "<wrapped>", contextWith(map[string]interface{}{
		"value": value,
	}))
}
