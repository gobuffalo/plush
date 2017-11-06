package cmd

import (
	"testing"
	"encoding/json"
	"io/ioutil"
	"os"
)

func TestParseContextVars(t *testing.T) {
	x := map[string]interface{}{}

	t.Run("Parsing should not fail for nil array", func(t *testing.T) {
		if err := parseContextVars(nil, x); err != nil {
			t.Error(err)
			return
		}

		if len(x) > 0 {
			t.Error("Parsing with nil array should keep map as empty")
			return
		}
	})

	values := []string{"x=y", "p=", "q"}
	if err := parseContextVars(values, x); err != nil {
		t.Fatal(err)
		return
	}

	t.Run("Valid Values should be appended", func(t *testing.T) {
		if len(x) != 2 {
			t.Error("Map should only have two keys, x and p")
		}
	})

	t.Run("Validate x's presence in the map", func(t *testing.T) {
		val, ok := x["x"]
		if !ok {
			t.Error("x should be present in the map")
		} else if val != "y" {
			t.Errorf("Value of x should be y. Found %v", val)
		}
	})

	t.Run("Validate p's presence in the map", func(t *testing.T) {
		val, ok := x["p"]
		if !ok {
			t.Error("p should be present in the map")
		} else if val != "" {
			t.Errorf("Value of p should be empty string. Found %v", val)
		}
	})

	t.Run("Invalid values should be omitted", func(t *testing.T) {
		for _, k := range []string{"q"} {
			if _, ok := x[k]; ok {
				t.Error("P should be missing in the parsed map")
				return
			}
		}
	})
}

func TestParseContextBytes(t *testing.T) {
	data := map[string]interface{}{"x": "y", "p": "q"}
	bytes, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
		return
	}

	x := map[string]interface{}{}
	if err := parseContextBytes(bytes, x); err != nil {
		t.Fatal(err)
		return
	}

	t.Run("Validate x's presence in the map", func(t *testing.T) {
		val, ok := x["x"]
		if !ok {
			t.Error("x should be present in the map")
		} else if val != "y" {
			t.Errorf("Value of x should be y. Found %v", val)
		}
	})

	t.Run("Validate p's presence in the map", func(t *testing.T) {
		val, ok := x["p"]
		if !ok {
			t.Error("p should be present in the map")
		} else if val != "q" {
			t.Errorf("Value of p should be 1. Found %v", val)
		}
	})
}

func TestParseContextFile(t *testing.T) {
	file, err := ioutil.TempFile(os.TempDir(), "prefix")
	if err != nil {
		t.Fatal(err)
		return
	}

	defer os.Remove(file.Name())

	data := map[string]interface{}{"x": "y", "p": "q"}
	bytes, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
		return
	}

	if _, err := file.Write(bytes); err != nil {
		t.Fatal(err)
		return
	}

	x := map[string]interface{}{}
	if err := parseContextFile(file.Name(), x); err != nil {
		t.Fatal(err)
		return
	}

	t.Run("Validate x's presence in the map", func(t *testing.T) {
		val, ok := x["x"]
		if !ok {
			t.Error("x should be present in the map")
		} else if val != "y" {
			t.Errorf("Value of x should be y. Found %v", val)
		}
	})

	t.Run("Validate p's presence in the map", func(t *testing.T) {
		val, ok := x["p"]
		if !ok {
			t.Error("p should be present in the map")
		} else if val != "q" {
			t.Errorf("Value of p should be 1. Found %v", val)
		}
	})
}

func TestRenderTmpl(t *testing.T) {
	t.Run("Should complain about missing plush template", func(t *testing.T) {
		err := renderCmd.RunE(nil, []string{})
		if err.Error() != "Must provide a plush template file" {
			t.Error("Should raise missing template file error")
			return
		}
	})

	t.Run("Render without arguments", func(t *testing.T) {
		out, err := renderTmpl("./testdata/greeting.plush", "", nil)
		if err != nil {
			t.Error(err)
		}

		if out != "Hello, " {
			t.Error("Expected output mismatch")
			return
		}
	})

	t.Run("Render with arguments", func(t *testing.T) {
		out, err := renderTmpl("./testdata/greeting.plush", "", []string{"name=piyush"})
		if err != nil {
			t.Error(err)
		}

		if out != "Hello, piyush" {
			t.Error("Expected output mismatch")
			return
		}
	})

	t.Run("Render with File", func(t *testing.T) {
		ctxFile, err := ioutil.TempFile(os.TempDir(), "prefix")
		if err != nil {
			t.Fatal(err)
			return
		}

		defer os.Remove(ctxFile.Name())

		data := map[string]interface{}{"name": "Piyush"}
		bytes, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
			return
		}

		if _, err := ctxFile.Write(bytes); err != nil {
			t.Fatal(err)
			return
		}

		t.Run("Run without variables override", func(t *testing.T) {
			out, err := renderTmpl("./testdata/greeting.plush", ctxFile.Name(), []string{})
			if err != nil {
				t.Error(err)
			}

			if out != "Hello, Piyush" {
				t.Error("Expected output mismatch")
				return
			}
		})

		t.Run("Run with variables override", func(t *testing.T) {
			out, err := renderTmpl("./testdata/greeting.plush", ctxFile.Name(), []string{"name=meson10"})
			if err != nil {
				t.Error(err)
			}

			if out != "Hello, meson10" {
				t.Error("Expected output mismatch")
				return
			}
		})
	})
}