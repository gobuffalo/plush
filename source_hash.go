package plush

import (
	"hash/maphash"
	"strconv"
)

var templateSourceHashSeed = maphash.MakeSeed()

func templateSourceHash(input string) string {
	return strconv.FormatUint(maphash.String(templateSourceHashSeed, input), 16)
}

func templateSourceCacheHash(input string) string {
	if input == "" {
		return ""
	}
	return templateSourceHash(input)
}

func templateSourceMatches(t *Template, sourceHash string) bool {
	return t != nil && (sourceHash == "" || t.SourceHash == "" || t.SourceHash == sourceHash)
}
