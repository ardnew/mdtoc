package main

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

func testBuildInfo() *debug.BuildInfo {
	return &debug.BuildInfo{
		Main: debug.Module{
			Path:    "github.com/ardnew/mdtoc",
			Version: appVersion(),
		},
	}
}

func TestGenerateTOC(t *testing.T) {
	toc, err := generateTOC([]source{{name: "guide.md", reader: strings.NewReader("# Main\n### Child\n## [Link](x) &amp; `code`\n")}}, options{prefix: "."})
	if err != nil {
		t.Fatal(err)
	}
	want := "- [Main](#main)\n  - [Child](#child)\n  - [Link & code](#link--code)\n"
	if string(toc) != want {
		t.Errorf("generateTOC() = %q, want %q", toc, want)
	}
}

func TestFiltersAndLevels(t *testing.T) {
	levels, err := parseLevels("2:3")
	if err != nil {
		t.Fatal(err)
	}
	filters, err := parseFilters([]string{"/keep/i", "!/drop/"})
	if err != nil {
		t.Fatal(err)
	}
	toc, err := generateTOC([]source{{name: "x.md", reader: strings.NewReader("# keep\n## Keep two\n### drop keep\n#### keep four\n")}}, options{levels: levels, filters: filters})
	if err != nil {
		t.Fatal(err)
	}
	want := "- [Keep two](#keep-two)\n"
	if string(toc) != want {
		t.Errorf("generateTOC() = %q, want %q", toc, want)
	}
}

func TestInsertTOC(t *testing.T) {
	got, err := insertTOC([]byte("start\r\n<!-- mdtoc -->\r\nold\r\n<!-- mdtoc -->\r\nend\r\n"), "mdtoc", []byte("- [One](#one)\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := "start\r\n<!-- mdtoc -->\r\n- [One](#one)\r\n<!-- mdtoc -->\r\nend\r\n"
	if string(got) != want {
		t.Errorf("insertTOC() = %q, want %q", got, want)
	}
}

func TestInsertTOCNormalizesIrregularMarkerSpacing(t *testing.T) {
	for _, input := range []string{
		"<!--mdtoc-->",
		"<!--mdtoc -->",
		"<!-- mdtoc-->",
		"<!--     mdtoc-->",
		"<!--mdtoc             -->",
	} {
		t.Run(input, func(t *testing.T) {
			document := []byte("before\n" + input + "\nold\n" + input + "\nafter\n")
			got, err := insertTOC(document, "mdtoc", []byte("- [One](#one)\n"))
			if err != nil {
				t.Fatal(err)
			}
			want := "before\n<!-- mdtoc -->\n- [One](#one)\n<!-- mdtoc -->\nafter\n"
			if string(got) != want {
				t.Errorf("insertTOC() = %q, want %q", got, want)
			}
		})
	}
}

func TestInsertTOCNormalizesLoneMarker(t *testing.T) {
	got, err := insertTOC([]byte("before\n<!--mdtoc-->\nafter\n"), "mdtoc", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "before\n<!-- mdtoc -->\n<!-- mdtoc -->\nafter\n"
	if string(got) != want {
		t.Errorf("insertTOC() = %q, want %q", got, want)
	}
}

func TestRunCLI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI(nil, testBuildInfo(), strings.NewReader("# Title\n"), &stdout, &stderr)
	if code != 0 || stdout.String() != "- [Title](#title)\n" || stderr.Len() != 0 {
		t.Errorf("runCLI = (%d, %q, %q)", code, stdout.String(), stderr.String())
	}
}

func TestRunCLIHelpIncludesDescription(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--help"}, testBuildInfo(), strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCLI help status = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), commandDescription) {
		t.Fatalf("help output = %q, want description %q", stdout.String(), commandDescription)
	}
	if !strings.Contains(stdout.String(), "mdtoc version "+appVersion()) {
		t.Fatalf("help output = %q, want version %q", stdout.String(), appVersion())
	}
	if stderr.Len() != 0 {
		t.Fatalf("help stderr = %q, want empty", stderr.String())
	}
}

func TestRunCLIUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--usage"}, testBuildInfo(), strings.NewReader(""), &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("runCLI usage = (%d, %q)", code, stderr.String())
	}
	for _, text := range []string{"<!-- contents -->", "--insert guide.md --mark contents", "--write"} {
		if !strings.Contains(stdout.String(), text) {
			t.Errorf("usage output missing %q", text)
		}
	}
}

func TestRunCLIVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--version"}, testBuildInfo(), strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCLI version status = %d, want 0", code)
	}
	if got, want := strings.TrimSpace(stdout.String()), appVersion(); got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("version stderr = %q, want empty", stderr.String())
	}
}

func TestLevelParsingRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", ":", "1:", ":1", "1:2:3", "-1", "+1", "7", "3:2", " 1"} {
		if _, err := parseLevels(value); err == nil {
			t.Errorf("parseLevels(%q) succeeded", value)
		}
	}
}

func TestFragmentsAreAllocatedBeforeFiltering(t *testing.T) {
	levels, err := parseLevels("3:3")
	if err != nil {
		t.Fatal(err)
	}
	toc, err := generateTOC([]source{{name: "x", reader: strings.NewReader("# duplicate\n### duplicate\n")}}, options{levels: levels})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(toc), "- [duplicate](#duplicate-1)\n"; got != want {
		t.Errorf("generateTOC() = %q, want %q", got, want)
	}
}

func TestInsertIgnoresCodeMarkersAndKeepsLaterPairs(t *testing.T) {
	input := []byte("<!-- mdtoc -->\n```\n<!-- mdtoc -->\n```\n<!-- mdtoc -->\nkeep\n<!-- mdtoc -->\n")
	got, err := insertTOC(input, "mdtoc", []byte("- [A](#a)\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := "<!-- mdtoc -->\n- [A](#a)\n<!-- mdtoc -->\nkeep\n<!-- mdtoc -->\n"
	if string(got) != want {
		t.Errorf("insertTOC() = %q, want %q", got, want)
	}
}
