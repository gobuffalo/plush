package plush

import "strings"

func preprocessTrimTags(input string) string {
	if !strings.Contains(input, "<%-") {
		return input
	}

	out := make([]byte, 0, len(input))
	for i := 0; i < len(input); {
		if strings.HasPrefix(input[i:], "<%-") {
			out = trimWhitespaceSuffix(out)
			out = append(out, "<%="...)
			i += len("<%-")

			end := strings.Index(input[i:], "%>")
			if end < 0 {
				out = append(out, input[i:]...)
				return string(out)
			}

			out = append(out, input[i:i+end+len("%>")]...)
			i += end + len("%>")
			for i < len(input) && isTrimWhitespace(input[i]) {
				i++
			}
			continue
		}

		out = append(out, input[i])
		i++
	}

	return string(out)
}

func trimWhitespaceSuffix(input []byte) []byte {
	for len(input) > 0 && isTrimWhitespace(input[len(input)-1]) {
		input = input[:len(input)-1]
	}
	return input
}

func isTrimWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}
