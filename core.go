package main

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
)

type source struct {
	name   string
	reader io.Reader
	stdin  bool
}

type levelRange struct{ min, max int }

type filter struct {
	exclude bool
	re      *regexp.Regexp
}

type options struct {
	prefix  string
	levels  levelRange
	filters []filter
}

type heading struct {
	title    string
	fragment string
	level    int
	source   source
	sourceID int
}

func parseLevels(value string) (levelRange, error) {
	parts := strings.Split(value, ":")
	if len(parts) > 2 || len(parts) == 0 || value == "" {
		return levelRange{}, fmt.Errorf("level must have the form [MIN:]MAX")
	}
	parse := func(s string) (int, error) {
		if s == "" {
			return 0, fmt.Errorf("level values must not be empty")
		}
		for _, r := range s {
			if r < '0' || r > '9' {
				return 0, fmt.Errorf("level values must be decimal integers from 0 through 6")
			}
		}
		n, _ := strconv.Atoi(s)
		if n > 6 {
			return 0, fmt.Errorf("level values must be from 0 through 6")
		}
		return n, nil
	}
	if len(parts) == 1 {
		max, err := parse(parts[0])
		return levelRange{max: max}, err
	}
	min, err := parse(parts[0])
	if err != nil {
		return levelRange{}, err
	}
	max, err := parse(parts[1])
	if err != nil {
		return levelRange{}, err
	}
	if min != 0 && max != 0 && min > max {
		return levelRange{}, fmt.Errorf("minimum level must not exceed maximum level")
	}
	return levelRange{min: min, max: max}, nil
}

func parseFilters(values []string) ([]filter, error) {
	result := make([]filter, 0, len(values))
	for _, value := range values {
		f := filter{}
		if strings.HasPrefix(value, "!") {
			f.exclude, value = true, value[1:]
		}
		if !strings.HasPrefix(value, "/") {
			return nil, fmt.Errorf("filter must have the form [!]/PATTERN/[i]")
		}
		last := strings.LastIndex(value, "/")
		if last == 0 {
			return nil, fmt.Errorf("filter must have a closing slash")
		}
		suffix := value[last+1:]
		if suffix != "" && suffix != "i" {
			return nil, fmt.Errorf("filter has an invalid suffix")
		}
		pattern := value[1:last]
		if suffix == "i" {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid filter %q: %w", value, err)
		}
		f.re = re
		result = append(result, f)
	}
	return result, nil
}

func generateTOC(sources []source, opts options) ([]byte, error) {
	var selected []heading
	for sourceID, src := range sources {
		data, err := io.ReadAll(src.reader)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", src.name, err)
		}
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("%s: input is not valid UTF-8", src.name)
		}
		headings := parseHeadings(src, data)
		used := map[string]int{}
		for i := range headings {
			headings[i].sourceID = sourceID
			headings[i].fragment = allocateFragment(headings[i].title, used)
			if eligible(headings[i], opts) {
				selected = append(selected, headings[i])
			}
		}
	}
	return renderTOC(selected, opts.prefix)
}

func parseHeadings(src source, data []byte) []heading {
	doc := parser.New().Parse(data)
	var headings []heading
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := node.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		title := canonicalTitle(h, data)
		if title != "" {
			headings = append(headings, heading{title: title, level: h.Level, source: src})
		}
		return ast.WalkSkipChildren, nil
	})
	return headings
}

func canonicalTitle(h *ast.Heading, source []byte) string {
	var text strings.Builder
	var appendText func(ast.Node)
	appendText = func(node ast.Node) {
		switch n := node.(type) {
		case *ast.Text:
			text.WriteString(n.Value.Str(source))
			if n.SoftLineBreak() || n.HardLineBreak() {
				text.WriteByte(' ')
			}
		case *ast.CodeSpan:
			text.WriteString(n.Value.Str(source))
		case *ast.AutoLink:
			text.WriteString(n.Label.Str(source))
		case *ast.RawHTML:
			return
		default:
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				appendText(child)
			}
		}
	}
	for child := h.FirstChild(); child != nil; child = child.NextSibling() {
		appendText(child)
	}
	return strings.Join(strings.Fields(html.UnescapeString(text.String())), " ")
}

func allocateFragment(title string, used map[string]int) string {
	var slug strings.Builder
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || r == '_' || r == '-' || r == '\u200c' || r == '\u200d' {
			slug.WriteRune(r)
		} else if r == ' ' {
			slug.WriteByte('-')
		}
	}
	base := slug.String()
	if base == "" {
		base = "heading"
	}
	if _, found := used[base]; !found {
		used[base] = 1
		return base
	}
	for n := used[base]; ; n++ {
		candidate := base + "-" + strconv.Itoa(n)
		if _, found := used[candidate]; !found {
			used[base] = n + 1
			used[candidate] = 1
			return candidate
		}
	}
}

func eligible(h heading, opts options) bool {
	if opts.levels.min != 0 && h.level < opts.levels.min || opts.levels.max != 0 && h.level > opts.levels.max {
		return false
	}
	included := false
	hasInclusion := false
	for _, f := range opts.filters {
		if f.exclude {
			if f.re.MatchString(h.title) {
				return false
			}
		} else {
			hasInclusion = true
			included = included || f.re.MatchString(h.title)
		}
	}
	return !hasInclusion || included
}

func renderTOC(headings []heading, prefix string) ([]byte, error) {
	var output bytes.Buffer
	var stack []heading
	previous := -1
	for _, h := range headings {
		if previous != h.sourceID {
			stack = nil
			previous = h.sourceID
		}
		for len(stack) > 0 && stack[len(stack)-1].level >= h.level {
			stack = stack[:len(stack)-1]
		}
		destination, err := linkDestination(prefix, h.source, h.fragment)
		if err != nil {
			return nil, err
		}
		output.WriteString(strings.Repeat("  ", len(stack)))
		output.WriteString("- [")
		output.WriteString(escapeLabel(h.title))
		output.WriteString("](")
		output.WriteString(destination)
		output.WriteString(")\n")
		stack = append(stack, h)
	}
	return output.Bytes(), nil
}

func escapeLabel(s string) string {
	return strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "\n", " ").Replace(s)
}

func linkDestination(prefix string, src source, fragment string) (string, error) {
	fragment = url.QueryEscape(fragment)
	fragment = strings.ReplaceAll(fragment, "+", "%20")
	if src.stdin || prefix == "." || prefix == "" || prefix == "#" {
		return "#" + fragment, nil
	}
	base, err := url.Parse(prefix)
	if err != nil || base.Opaque != "" || base.RawQuery != "" || base.Fragment != "" {
		return "", fmt.Errorf("prefix must be a non-opaque URI without a query or fragment")
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	name := strings.ReplaceAll(src.name, "\\", "/")
	ref := &url.URL{Path: name}
	resolved := base.ResolveReference(ref)
	resolved.Fragment = fragment
	return resolved.String(), nil
}
