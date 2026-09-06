package main

import (
	"bytes"
	"fmt"
	"strings"
)

type markerLine struct{ start, end int }

func validateMark(mark string) error {
	if mark == "" || strings.TrimSpace(mark) != mark || strings.ContainsAny(mark, "\r\n<>") || strings.Contains(mark, "--") {
		return fmt.Errorf("mark must be nonempty and must not contain whitespace at its ends, line endings, <, >, or --")
	}
	return nil
}

func insertTOC(document []byte, mark string, toc []byte) ([]byte, error) {
	marker := "<!-- " + mark + " -->"
	var matches []markerLine
	inFence := false
	for pos := 0; pos < len(document); {
		next := bytes.IndexByte(document[pos:], '\n')
		end := len(document)
		if next >= 0 {
			end = pos + next + 1
		}
		lineEnd := end
		if lineEnd > pos && document[lineEnd-1] == '\n' {
			lineEnd--
		}
		if lineEnd > pos && document[lineEnd-1] == '\r' {
			lineEnd--
		}
		line := string(document[pos:lineEnd])
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
		} else if !inFence && !isIndentedCodeLine(line) && isMarkerLine(line, mark) {
			matches = append(matches, markerLine{start: pos, end: end})
		}
		if next < 0 {
			break
		}
		pos = end
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("cannot find marker %q; add the marker to the document or select another value with --mark", marker)
	}
	open := matches[0]
	var close *markerLine
	if len(matches) > 1 {
		close = &matches[1]
	}
	eol := "\n"
	if open.end-open.start >= 2 && document[open.end-2] == '\r' && document[open.end-1] == '\n' {
		eol = "\r\n"
	}
	normalized := bytes.ReplaceAll(toc, []byte("\r\n"), []byte("\n"))
	normalized = bytes.ReplaceAll(normalized, []byte("\n"), []byte(eol))
	if len(normalized) > 0 && !bytes.HasSuffix(normalized, []byte(eol)) {
		normalized = append(normalized, eol...)
	}
	var out bytes.Buffer
	out.Write(document[:open.start])
	out.WriteString(marker)
	out.WriteString(eol)
	out.Write(normalized)
	if close != nil {
		out.WriteString(marker)
		out.WriteString(eol)
		out.Write(document[close.end:])
	} else {
		out.WriteString(marker)
		out.WriteString(eol)
		out.Write(document[open.end:])
	}
	return out.Bytes(), nil
}

func isMarkerLine(line, mark string) bool {
	if !strings.HasPrefix(line, "<!--") || !strings.HasSuffix(line, "-->") {
		return false
	}
	inner := strings.TrimSpace(line[len("<!--") : len(line)-len("-->")])
	return inner == mark
}

func isIndentedCodeLine(line string) bool {
	return strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ")
}
