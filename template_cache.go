package plush

import (
	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

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

func CachedVMBytecode(filename string, ctx hctx.Context) (interface{}, bool) {
	if filename == "" || !cacheEnabled || templateCacheBackend == nil || !isVMBytecodeCacheableFile(filename) || isHole(ctx) {
		return nil, false
	}
	return CachedVMBytecodeForFilename(filename)
}

func CachedVMBytecodeForFilename(filename string) (interface{}, bool) {
	return CachedVMBytecodeForFilenameWithSource(filename, "")
}

func CachedVMBytecodeForFilenameWithSource(filename string, source string) (interface{}, bool) {
	if filename == "" || !cacheEnabled || templateCacheBackend == nil || !isVMBytecodeCacheableFile(filename) {
		return nil, false
	}
	t, ok := templateCacheBackend.Get(GenerateASTKey(filename))
	if !ok || t == nil || t.VMBytecode == nil || !templateSourceMatches(t, templateSourceCacheHash(source)) {
		return nil, false
	}
	return t.VMBytecode, true
}

func CachedVMBytecodeForCleanFilename(filename string) (interface{}, bool) {
	return CachedVMBytecodeForCleanFilenameWithSource(filename, "")
}

func CachedVMBytecodeForCleanFilenameWithSource(filename string, source string) (interface{}, bool) {
	if filename == "" || !cacheEnabled || templateCacheBackend == nil || !isVMBytecodeCacheableFile(filename) {
		return nil, false
	}
	t, ok := templateCacheBackend.Get(GenerateASTKeyFromCleanFilename(filename))
	if !ok || t == nil || t.VMBytecode == nil || !templateSourceMatches(t, templateSourceCacheHash(source)) {
		return nil, false
	}
	return t.VMBytecode, true
}

func CachedASTProgram(filename string, ctx hctx.Context) (*ast.Program, bool) {
	return CachedASTProgramWithSource(filename, ctx, "")
}

func CachedASTProgramWithSource(filename string, ctx hctx.Context, source string) (*ast.Program, bool) {
	if filename == "" || !cacheEnabled || templateCacheBackend == nil || !isFilePlush(filename) || isHole(ctx) {
		return nil, false
	}
	t, ok := templateCacheBackend.Get(GenerateASTKey(filename))
	if !ok || t == nil || t.Program == nil || !templateSourceMatches(t, templateSourceCacheHash(source)) {
		return nil, false
	}
	return t.Program, true
}

func CacheVMBytecode(filename string, ctx hctx.Context, program *ast.Program, bytecode interface{}) {
	cacheVMBytecode(filename, ctx, program, bytecode)
}

func cacheVMBytecode(filename string, ctx hctx.Context, program *ast.Program, bytecode interface{}) {
	if filename == "" || !cacheEnabled || templateCacheBackend == nil || !isVMBytecodeCacheableFile(filename) || isHole(ctx) || bytecode == nil {
		return
	}
	CacheVMBytecodeForFilename(filename, program, bytecode)
}

func CacheVMBytecodeForFilename(filename string, program *ast.Program, bytecode interface{}) {
	CacheVMBytecodeForFilenameWithSource(filename, program, bytecode, "")
}

func CacheVMBytecodeForFilenameWithSource(filename string, program *ast.Program, bytecode interface{}, source string) {
	if filename == "" || !cacheEnabled || templateCacheBackend == nil || !isVMBytecodeCacheableFile(filename) || bytecode == nil {
		return
	}

	key := GenerateASTKey(filename)
	cacheVMBytecodeWithKey(key, program, bytecode, templateSourceCacheHash(source))
}

func CacheVMBytecodeForCleanFilename(filename string, program *ast.Program, bytecode interface{}) {
	CacheVMBytecodeForCleanFilenameWithSource(filename, program, bytecode, "")
}

func CacheVMBytecodeForCleanFilenameWithSource(filename string, program *ast.Program, bytecode interface{}, source string) {
	if filename == "" || !cacheEnabled || templateCacheBackend == nil || !isVMBytecodeCacheableFile(filename) || bytecode == nil {
		return
	}
	cacheVMBytecodeWithKey(GenerateASTKeyFromCleanFilename(filename), program, bytecode, templateSourceCacheHash(source))
}

func cacheVMBytecodeWithKey(key string, program *ast.Program, bytecode interface{}, sourceHash string) {
	t, ok := templateCacheBackend.Get(key)
	if !ok || t == nil || !templateSourceMatches(t, sourceHash) {
		t = &Template{}
	} else {
		cloned := *t
		t = &cloned
	}
	if program != nil {
		t.Program = program
	}
	t.Input = ""
	t.VMBytecode = bytecode
	t.SourceHash = sourceHash
	t.IsCache = false
	templateCacheBackend.Set(key, t)
}
