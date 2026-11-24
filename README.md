# mdtoc

`mdtoc` makes a Markdown table of contents from CommonMark headings.

## Usage

```text
mdtoc [FLAGS] [PATH...]
```

Run `mdtoc document.md` to write a table of contents to standard output.
Run `mdtoc --usage` to see an example session.
Run `mdtoc` with no paths to read standard input. Use `--` before a path that
starts with `-`.

```sh
mdtoc guide.md
mdtoc --prefix ./docs guide.md
mdtoc --insert README.md --mark mdtoc guide.md
mdtoc --insert README.md --write
```

Use two identical full-line markers to replace the content between them:

```md
<!-- mdtoc -->
<!-- mdtoc -->
```

With `--insert`, `mdtoc` writes the complete changed document to standard
output. Add `--write` to replace the insertion document atomically. `--write`
and an explicit `--mark` require `--insert`.

## Flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--usage` | | | Show an example session. |
| `--prefix URI` | `-p URI` | `.` | Set a prefix for file links. |
| `--insert PATH` | `-I PATH` | | Set the insertion document. |
| `--mark TEXT` | `-m TEXT` | `mdtoc` | Set marker text. |
| `--level [MIN:]MAX` | `-l [MIN:]MAX` | `0:0` | Select heading levels. |
| `--filter SPEC` | `-f SPEC` | | Select headings with a regular expression. |
| `--write` | `-w` | off | Replace the insertion document. |

Filter specifications use `[!]/PATTERN/[i]`. Use `!` to exclude a matching
heading. More than one inclusion filter uses OR. An exclusion filter always
rejects a matching heading.

The command returns zero on success, two for command syntax and flag errors,
and one for input, insertion, and output errors.
