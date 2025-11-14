package plush

import (
	"fmt"
	"html/template"
	"path/filepath"
	"strings"
	"time"

	"github.com/gobuffalo/plush/v5/helpers/meta"
)

// already_in_partial is a unique key used in the helper context to track whether
// the current execution is already inside a partial template. This helps prevent
// recursive or redundant partial rendering. The value is constructed with a unique
// suffix to avoid key collisions with user-defined variables within a single program run.
var already_in_partial = "__plush_internal_already_in_partial_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "__"

// PartialFeeder is callback function should implemented on application side.
type PartialFeeder func(string) (string, error)

func PartialHelper(name string, data map[string]interface{}, help HelperContext) (template.HTML, error) {
	if help.Context == nil {
		return "", fmt.Errorf("invalid context. abort")
	}

	help.Context = help.New()
	for k, v := range data {
		help.Set(k, v)
	}
	base := help.Value(meta.TemplateBaseFileNameKey)
	ext := help.Value(meta.TemplateExtensionKey)
	fileKey := help.Value(meta.TemplateFileKey)
	if base != nil && fileKey != nil && ext != nil {
		templateFileKey, ok := fileKey.(string)
		if !ok {
			return "", fmt.Errorf("expected fileKey to be a string, got %T", fileKey)
		}
		if v := help.Value(already_in_partial); v != nil {
			if parentPartial, ok := v.(string); ok {
				// If already rendering a partial, remove the parent partial's name from the end of the template file key.
				templateFileKey = strings.TrimSuffix(templateFileKey, parentPartial)
			}
		}
		baseVal, baseOk := base.(string)
		extVal, extOk := ext.(string)
		if baseOk && extOk {
			constructFileName := baseVal + "." + extVal
			// Remove the base template filename (with extension) from the end of the template file key.
			templateFileKey = strings.TrimSuffix(templateFileKey, constructFileName)
		}
		// Construct the new template file key by joining the (possibly trimmed) path with the partial name.
		help.Set(meta.TemplateFileKey, strings.ReplaceAll(filepath.Join(templateFileKey, name), "\\", "/"))
	} else {
		help.Set(meta.TemplateFileKey, name)
	}

	pf, ok := help.Value("partialFeeder").(func(string) (string, error))
	if !ok {
		return "", fmt.Errorf("could not find partial feeder from helpers")
	}

	var part string
	var err error
	if part, err = pf(name); err != nil {
		return "", err
	}
	if help.Value(already_in_partial) == nil {
		help.Set(already_in_partial, name)
		defer help.Set(already_in_partial, nil)
	} else {
		// Save original values to restore after rendering the partial
		origBase := help.Value(meta.TemplateBaseFileNameKey)
		origExt := help.Value(meta.TemplateExtensionKey)
		extNm := filepath.Ext(name)
		help.Set(meta.TemplateBaseFileNameKey, strings.TrimSuffix(name, extNm))
		help.Set(meta.TemplateExtensionKey, strings.TrimPrefix(extNm, "."))
		defer func() {
			help.Set(meta.TemplateBaseFileNameKey, origBase)
			help.Set(meta.TemplateExtensionKey, origExt)
		}()
	}
	if part, err = Render(part, help.Context); err != nil {
		return "", err
	}
	if ct, ok := help.Value("contentType").(string); ok {
		ext := filepath.Ext(name)
		if strings.Contains(ct, "javascript") && ext != ".js" && ext != "" {
			part = template.JSEscapeString(string(part))
		}
	}

	if layout, ok := data["layout"].(string); ok {
		return PartialHelper(
			layout,
			map[string]interface{}{"yield": template.HTML(part)},
			help)
	}

	return template.HTML(part), err
}
