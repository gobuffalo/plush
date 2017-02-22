package vv

import "github.com/pkg/errors"

// HelperMap holds onto helpers and validates they are properly formed.
type HelperMap struct {
	helpers map[string]interface{}
}

// NewHelperMap containing all of the "default" helpers from "vv.Helpers".
func NewHelperMap() (HelperMap, error) {
	hm := HelperMap{
		helpers: map[string]interface{}{},
	}

	err := hm.AddMany(Helpers.Helpers())
	if err != nil {
		return hm, errors.WithStack(err)
	}
	return hm, nil
}

// Add a new helper to the map. New Helpers will be validated to ensure they
// meet the requirements for a helper:
/*
	func(...) (string) {}
	func(...) (string, error) {}
	func(...) (template.HTML) {}
	func(...) (template.HTML, error) {}
*/
func (h *HelperMap) Add(key string, helper interface{}) error {
	if h.helpers == nil {
		h.helpers = map[string]interface{}{}
	}
	h.helpers[key] = helper
	return nil
}

// AddMany helpers at the same time.
func (h *HelperMap) AddMany(helpers map[string]interface{}) error {
	for k, v := range helpers {
		err := h.Add(k, v)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

// Helpers returns the underlying list of helpers from the map
func (h HelperMap) Helpers() map[string]interface{} {
	return h.helpers
}
