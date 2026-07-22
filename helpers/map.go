package helpers

import (
	"sync"
	"sync/atomic"
)

// HelperMap holds onto helpers and validates they are properly formed.
type HelperMap struct {
	helpers map[string]interface{}
	moot    *sync.Mutex
	version uint64
	latest  uint64
}

// NewHelperMap containing all of the "default" helpers from "plush.Helpers".
func NewMap(helpers map[string]interface{}) HelperMap {
	hm := HelperMap{
		helpers: helpers,
		moot:    &sync.Mutex{},
	}

	return hm
}

// Add a new helper to the map. New Helpers will be validated to ensure they
// meet the requirements for a helper:
func (h *HelperMap) Add(key string, helper interface{}) error {
	h.moot.Lock()
	defer h.moot.Unlock()

	if h.helpers == nil {
		h.helpers = map[string]interface{}{}
	}

	h.helpers[key] = helper
	h.version++
	atomic.StoreUint64(&h.latest, h.version)

	return nil
}

// AddMany helpers at the same time.
func (h *HelperMap) AddMany(helpers map[string]interface{}) error {
	for k, v := range helpers {
		err := h.Add(k, v)
		if err != nil {
			return err
		}
	}

	return nil
}

// Helpers returns the underlying list of helpers from the map
func (h HelperMap) Helpers() map[string]interface{} {
	return h.helpers
}

func (h HelperMap) All() map[string]interface{} {
	return h.helpers
}

func (h *HelperMap) Version() uint64 {
	return atomic.LoadUint64(&h.latest)
}

func (h *HelperMap) Snapshot() (map[string]interface{}, uint64) {
	h.moot.Lock()
	defer h.moot.Unlock()

	out := make(map[string]interface{}, len(h.helpers))
	for k, v := range h.helpers {
		out[k] = v
	}
	return out, h.version
}
