// Package htmlmut provides helper functions around regexes that help modify
// HTML documents.
package htmlmut

import (
	"bytes"
	"fmt"
	"regexp"
)

type MutateFunc = func([]byte) []byte

func ChainMutators(mutators ...MutateFunc) MutateFunc {
	return func(html []byte) []byte {
		for _, mutator := range mutators {
			html = mutator(html)
		}
		return html
	}
}

// TagRemover creates a regex-based HTML tag remover.
func TagRemover(tag string) MutateFunc {
	re := regexp.MustCompile(fmt.Sprintf(
		`(?mU) *<%[1]s(.|\n)+</%[1]s>`,
		tag,
	))

	return func(html []byte) []byte {
		return re.ReplaceAll(html, nil)
	}
}

var bodyOpeningRegex = regexp.MustCompile("<body.*>")

// ScriptInjector injects raw JavaScript to the front of the body block.
func ScriptInjector(script string) MutateFunc {
	scriptTag := []byte(fmt.Sprintf("<body>\n\n<script>%s</script>\n\n", script))

	return func(html []byte) []byte {
		return bodyOpeningRegex.ReplaceAll(html, scriptTag)
	}
}

var headOpeningRegex = regexp.MustCompile("<head.*>")

// ExternScriptInjector injects an external JavaScript file to the front of the
// head block.
func ExternScriptInjector(src string) MutateFunc {
	builder := bytes.Buffer{}
	builder.WriteString("<head>\n\n")
	builder.WriteString(`<script src="`)
	builder.WriteString(src)
	builder.WriteString(`"></script>`)

	return func(html []byte) []byte {
		return headOpeningRegex.ReplaceAll(html, builder.Bytes())
	}
}

var headClosingRegex = regexp.MustCompile("</head>")

// StylesheetInjector injects a CSS stylesheet at the end of the head block.
func StylesheetInjector(cssLink string) MutateFunc {
	linkHead := fmt.Sprintf(
		`<link rel="stylesheet" href="%s">`+"\n\n</head>",
		cssLink,
	)

	return func(html []byte) []byte {
		return headClosingRegex.ReplaceAll(html, []byte(linkHead))
	}
}

// CSSInjector injects CSS at the end of the head block.
func CSSInjector(css string) MutateFunc {
	styleHead := fmt.Sprintf("<style>%s</style>\n\n</head>", css)

	return func(html []byte) []byte {
		return headClosingRegex.ReplaceAll(html, []byte(styleHead))
	}
}

// TagAttrReplace replaces attribute values of a given tag and attribute key.
func TagAttrReplace(tag, attr string, replacer func(string) string) MutateFunc {
	// $1 and $3 are the surrounding texts, and $2 is the link.
	regex := regexp.MustCompile(fmt.Sprintf(`(?mU)(<%s.+%s=")([^"]+)(".*>)`, tag, attr))

	return func(html []byte) []byte {
		matches := regex.FindAllSubmatch(html, -1)
		if matches == nil {
			return html
		}

		for _, match := range matches {
			// Replace directly the matched attribute.
			match[2] = []byte(replacer(string(match[2])))
			// We can raw replace the bytes directly. This might be a bit too
			// slow, but whatever.
			html = bytes.Replace(html, match[0], bytes.Join(match[1:], nil), 1)
		}

		return html
	}
}

// AnchorReplace replaces all hyperlinks in the HTML document.
func AnchorReplace(linkFunc func(string) string) MutateFunc {
	return TagAttrReplace("a", "href", linkFunc)
}
