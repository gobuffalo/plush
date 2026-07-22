package vm

import (
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/meta"
)

func newPartialBytecodeLinkCache() *partialBytecodeLinkCache {
	return &partialBytecodeLinkCache{entries: map[string]*partialBytecodeLink{}}
}

type partialMetaIDs struct {
	templateFileID     int
	templateBaseFileID int
	templateExtID      int
	alreadyPartialID   int
	contentTypeID      int
}

func (c *partialBytecodeLinkCache) Get(key string, sourceHash uint64) (*compiler.Bytecode, bool) {
	link, ok := c.GetLink(key, sourceHash)
	if !ok {
		return nil, false
	}
	return link.bytecode, true
}

func (c *partialBytecodeLinkCache) GetLink(key string, sourceHash uint64) (*partialBytecodeLink, bool) {
	if c == nil || key == "" {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok || entry.sourceHash != sourceHash || entry.bytecode == nil {
		return nil, false
	}
	return entry, true
}

func (c *partialBytecodeLinkCache) Set(key string, sourceHash uint64, bytecode *compiler.Bytecode) *partialBytecodeLink {
	return c.SetWithSource(key, sourceHash, "", bytecode)
}

func (c *partialBytecodeLinkCache) SetWithSource(key string, sourceHash uint64, source string, bytecode *compiler.Bytecode) *partialBytecodeLink {
	if c == nil || key == "" || bytecode == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	link := &partialBytecodeLink{sourceHash: sourceHash, source: source, bytecode: bytecode}
	c.entries[key] = link
	return link
}

func (c *partialBytecodeLinkCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *partialBytecodeLinkCache) partialFeeder(ctx hctx.Context) (func(string) (string, error), bool) {
	if ctx == nil {
		return nil, false
	}
	if lookup, ok := ctx.(contextIDLookup); ok && canCacheFastRenderBindingPlan(ctx) {
		id := c.partialFeederID(lookup)
		if value, ok := lookup.LookupID(id); ok {
			feeder, ok := value.(func(string) (string, error))
			return feeder, ok
		}
	}
	feeder, ok := ctx.Value(vmPartialFeederName).(func(string) (string, error))
	return feeder, ok
}

func (c *partialBytecodeLinkCache) partialFeederID(lookup contextIDLookup) int {
	if c == nil || lookup == nil {
		return -1
	}
	c.mu.RLock()
	if c.hasFeederID {
		id := c.feederID
		c.mu.RUnlock()
		return id
	}
	c.mu.RUnlock()

	id := lookup.InternID(vmPartialFeederName)
	c.mu.Lock()
	if !c.hasFeederID {
		c.feederID = id
		c.hasFeederID = true
	}
	id = c.feederID
	c.mu.Unlock()
	return id
}

func (c *partialBytecodeLinkCache) partialMetaIDs(ctx hctx.Context) (partialMetaIDs, bool) {
	if c == nil || !canCacheFastRenderBindingPlan(ctx) {
		return partialMetaIDs{}, false
	}
	lookup := ctx.(contextIDLookup)
	c.mu.RLock()
	if c.hasMetaIDs {
		ids := c.metaIDs
		c.mu.RUnlock()
		return ids, true
	}
	c.mu.RUnlock()

	ids := partialMetaIDs{
		templateFileID:     lookup.InternID(meta.TemplateFileKey),
		templateBaseFileID: lookup.InternID(meta.TemplateBaseFileNameKey),
		templateExtID:      lookup.InternID(meta.TemplateExtensionKey),
		alreadyPartialID:   lookup.InternID(vmAlreadyInPartial),
		contentTypeID:      lookup.InternID("contentType"),
	}
	c.mu.Lock()
	if !c.hasMetaIDs {
		c.metaIDs = ids
		c.hasMetaIDs = true
	}
	ids = c.metaIDs
	c.mu.Unlock()
	return ids, true
}

func (l *partialBytecodeLink) fastBindingPlan(ctx hctx.Context) *fastRenderBindingPlan {
	if l == nil || l.bytecode == nil || l.bytecode.FastRenderPlan == nil {
		return nil
	}
	l.mu.RLock()
	plan := l.bindingPlan
	l.mu.RUnlock()
	if plan != nil {
		return plan
	}
	plan = newFastRenderBindingPlan(l.bytecode.FastRenderPlan, ctx)
	if plan == nil {
		return nil
	}
	l.mu.Lock()
	if l.bindingPlan == nil {
		l.bindingPlan = plan
	}
	plan = l.bindingPlan
	l.mu.Unlock()
	return plan
}
