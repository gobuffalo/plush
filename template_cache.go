package plush

type TemplateCache interface {
	// Get retrieves a cached template by key.
	Get(key string) (*Template, bool)
	// Set stores a template in the cache.
	Set(key string, t *Template)
	// Delete removes a template from the cache.
	Delete(key ...string)

	Clear()
}

func ClearTemplateCache() {
	if templateCacheBackend != nil {
		templateCacheBackend.Clear()
	}
}
