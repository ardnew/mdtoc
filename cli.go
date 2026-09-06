package main

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"runtime/debug"
	"strings"

	"github.com/alecthomas/kong"
)

type command struct {
	Version kong.VersionFlag `help:"Show version and exit."`
	Prefix  string           `help:"Prefix for relative links." default:"." short:"p"`
	Insert  string           `help:"Path to insertion target document." short:"I"`
	Mark    string           `help:"Marker text in HTML comment." default:"${name}" short:"m"`
	Level   string           `help:"Heading level range to include." default:"0:0" short:"l"`
	Filter  []string         `help:"Include or exclude headings." short:"f"`
	Write   bool             `help:"Write result to insertion target file." short:"w"`
	Paths   []string         `arg:"" optional:"" name:"path"`
}

type cliExit int

const commandDescription = "Create a table of contents from Markdown headings"

func runCLI(args []string, info *debug.BuildInfo, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if exit, ok := recovered.(cliExit); ok {
				code = int(exit)
				return
			}
			panic(recovered)
		}
	}()

	vcs := map[string]string{}
	for _, setting := range info.Settings {
		if id, ok := strings.CutPrefix(setting.Key, "vcs."); ok {
			vcs[id] = setting.Value
		}
	}

	vars := kong.Vars{
		"name":    appName(info),
		"version": appVersion(),
	}

	ident := make([]string, 0, 5)
	ident = append(ident, vars["name"])
	ident = append(ident, "version", vars["version"])
	if rev, ok := vcs["revision"]; ok && rev != "" {
		vars["revision"] = rev
		ident = append(ident, "("+rev+")")
	}
	if date, ok := vcs["time"]; ok && date != "" {
		vars["date"] = date
		ident = append(ident, date)
	}

	var cmd command
	parser, err := kong.New(
		&cmd,
		kong.Name(vars["name"]),
		kong.Description(strings.Join(ident, " ")+"\n\n"+commandDescription),
		kong.ConfigureHelp(kong.HelpOptions{NoAppDescFormat: true}),
		kong.Writers(stdout, stderr),
		vars,
		kong.UsageOnError(),
		kong.Exit(func(code int) { panic(cliExit(code)) }),
	)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	_, err = parser.Parse(args)
	if err != nil {
		if err == nil {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 2
	}
	if cmd.Write && cmd.Insert == "" {
		fmt.Fprintln(stderr, "--write requires --insert")
		return 2
	}
	if hasFlag(args, "--mark", "-m") && cmd.Insert == "" {
		fmt.Fprintln(stderr, "--mark requires --insert")
		return 2
	}
	if err := validateMark(cmd.Mark); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	levels, err := parseLevels(cmd.Level)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	filters, err := parseFilters(cmd.Filter)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := validatePrefix(cmd.Prefix); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	var sources []source
	var insertion []byte
	if cmd.Insert != "" {
		insertion, err = readPath(cmd.Insert)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if len(cmd.Paths) == 0 {
		if cmd.Insert != "" {
			sources = append(sources, source{name: cmd.Insert, reader: bytes.NewReader(insertion)})
		} else {
			data, readErr := io.ReadAll(stdin)
			if readErr != nil {
				fmt.Fprintln(stderr, "stdin:", readErr)
				return 1
			}
			sources = append(sources, source{name: "stdin", reader: bytes.NewReader(data), stdin: true})
		}
	} else {
		for _, path := range cmd.Paths {
			data, readErr := readPath(path)
			if readErr != nil {
				fmt.Fprintln(stderr, readErr)
				return 1
			}
			sources = append(sources, source{name: path, reader: bytes.NewReader(data)})
		}
	}
	toc, err := generateTOC(sources, options{prefix: cmd.Prefix, levels: levels, filters: filters})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if cmd.Insert == "" {
		_, err = stdout.Write(toc)
	} else {
		var document []byte
		document, err = insertTOC(insertion, cmd.Mark, toc)
		if err != nil {
			err = fmt.Errorf("%s: %w", cmd.Insert, err)
		}
		if err == nil {
			if cmd.Write {
				err = atomicReplace(cmd.Insert, document)
			} else {
				_, err = stdout.Write(document)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func appVersion() string {
	return strings.TrimSpace(version)
}

func appName(info *debug.BuildInfo) string {
	if info == nil || info.Main.Path == "" {
		return "mdtoc"
	}
	return path.Base(info.Main.Path)
}

func hasFlag(args []string, long, short string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == long || arg == short || strings.HasPrefix(arg, long+"=") || (strings.HasPrefix(arg, short) && !strings.HasPrefix(arg, "--") && len(arg) > len(short)) {
			return true
		}
	}
	return false
}

func validatePrefix(prefix string) error {
	if prefix == "" || prefix == "." || prefix == "#" {
		return nil
	}
	u, err := url.Parse(prefix)
	if err != nil || u.Opaque != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("prefix must be a non-opaque URI without a query or fragment")
	}
	return nil
}

func readPath(path string) ([]byte, error) {
	u, err := url.Parse(path)
	if err == nil && u.IsAbs() {
		return nil, fmt.Errorf("%s: URLs are not input sources", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return data, nil
}
