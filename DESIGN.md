# DESIGN.md — mdtoc Requirements

This document specifies the authoritative requirements for `mdtoc`. All requirements use Simplified Technical English (ASD-STE100).

## 1. Command and Dependencies

### 1.1 Command Syntax

The command syntax is:

```
mdtoc [FLAGS] [PATH...]
```

Users provide zero or more paths as positional arguments.

### 1.2 Required Dependencies

`mdtoc` requires:

- Go 1.27 or later.
- Kong v1.16.1 for command-line parsing.
- Goldmark v2.0.1 for Markdown parsing.
- A local heading-slug implementation (no external slugger dependency).

### 1.3 Markdown Parsing

`mdtoc` must use the Goldmark CommonMark parser. `mdtoc` must not use optional Markdown extensions.

### 1.4 Architecture Constraints

- Developers must separate CLI adapters from processing logic.
- Processing logic must consume named `io.Reader` sources.
- Each successful operation must use one `io.Writer` content destination.
- Developers must treat stderr as a diagnostic destination. Stderr must not carry content.

### 1.5 Documentation and Standards

All future requirements, help text, errors, and documentation must follow ASD-STE100.

## 2. CLI Flags and Inputs

### 2.1 Flag Definitions

`mdtoc` provides these flags:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--prefix URI` | `-p URI` | `.` | Prefix for relative links. |
| `--insert PATH` | `-I PATH` | — | Path to insertion target document. |
| `--mark TEXT` | `-m TEXT` | `mdtoc` | Marker text in HTML comment. |
| `--level [MIN:]MAX` | `-l [MIN:]MAX` | `0:0` | Heading level range to include. |
| `--filter SPEC` | `-f SPEC` | — | Include or exclude headings (repeatable). |
| `--write` | `-w` | off | Write result to insertion target file. |

### 2.2 Input Path Rules

- Users must provide zero or more paths as positional arguments.
- Each positional argument must name one local readable file.
- URLs must not be input sources.
- The implementation must preserve positional argument order.
- The implementation must parse duplicate paths each time they occur.
- Users can use `--` to terminate flag parsing for paths that start with `-`.
- The implementation must treat a literal `-` as a normal filename. Users can write `./-` when needed.

### 2.3 Source Selection Rules

- When users provide no path argument and no insertion path, the implementation must use stdin.
- When `--insert` has no positional arguments, the implementation must use the insertion document as the implicit source.
- When users provide at least one path argument, the implementation must use each explicit positional path.
- The implementation must read all sources before content output starts.

### 2.4 Input Encoding and Validation

- The implementation must reject invalid UTF-8 with a source-specific error.
- The implementation must generate fragment-only links for stdin. The implementation must ignore `--prefix` for stdin links.
- The implementation must use the exact `--insert` argument as the logical input path for an implicit insertion source.

### 2.5 Flag Combination Rules

- When `--write` is used, the implementation must also require `--insert`.
- When `--mark` is used, the implementation must also require `--insert`.
- The implementation must reject invalid flag combinations as usage errors.

## 3. Headings, Selection, and Rendering

### 3.1 Heading Inclusion

The implementation must include ATX and Setext headings that Goldmark parses. The implementation must exclude raw HTML heading elements and code-block content.

### 3.2 Canonical Title Definition

The implementation must extract the canonical title from the heading AST. The implementation must follow these steps:

1. Extract visible text from the heading AST.
2. Decode character references.
3. Keep link labels, image alternative text, and code-span text.
4. Remove inline Markdown syntax and HTML tags.
5. Replace each whitespace sequence with one ASCII space.
6. Remove leading and trailing whitespace.
7. Exclude headings with an empty canonical title.

### 3.3 Level Parsing

The implementation must parse heading levels using this syntax:

```
[MIN:]MAX
```

The implementation must apply these rules:

- Use `0:0` as the default.
- When users provide `MAX` alone, interpret it as `0:MAX`.
- When users provide two values, require both values separated by one colon.
- Accept only integers from 0 through 6.
- Treat zero as removing that boundary.
- Reject a nonzero minimum above a nonzero maximum.
- Reject empty values, extra colons, signs, and nondecimal text.

### 3.4 Filter Parsing

Filter syntax is:

```
[!]/PATTERN/[i]
```

The implementation must apply these rules:

- Require the form `[!]/PATTERN/[i]`.
- Use the final slash as the closing delimiter.
- Permit unescaped slashes inside `PATTERN`.
- Treat `!` as an exclusion marker.
- Treat the final `i` as a case-insensitive flag.
- Use Go regular-expression syntax.
- Match against the canonical title.
- Combine multiple inclusion filters with OR.
- Reject any heading that matches any exclusion filter.
- Include all headings initially when no inclusion filter exists.
- Make filter order irrelevant.

### 3.5 Fragment Assignment Algorithm

The implementation must assign fragments before level and filter selection. The implementation must keep fragment allocation independent for each source document.

The implementation must follow this fragment algorithm:

1. Convert the canonical title to lowercase using Go Unicode rules.
2. Keep Unicode letters, numbers, and combining marks.
3. Keep `_`, `-`, U+200C (zero-width non-joiner), and U+200D (zero-width joiner).
4. Convert each canonical ASCII space to `-`.
5. Remove all other characters.
6. Use `heading` when the result is empty.
7. Track every parsed nonempty heading title.
8. Use the base fragment when it is unused.
9. Otherwise append `-1`, `-2`, and later integers until the fragment is unique.

### 3.6 URI Construction

The implementation must follow these URI construction rules:

- Convert filesystem separators to `/`.
- Percent-encode path data before URI resolution.
- Treat `--prefix` as a directory URI reference.
- Reject opaque prefixes and prefixes with a query or fragment.
- Resolve relative input paths using RFC 3986 rules.
- Preserve absolute-path behavior from RFC 3986.
- Apply no physical path resolution during link construction.
- Append the encoded heading fragment after URI resolution.

URI construction examples:

- Prefix `.`: Use the heading fragment only.
- Prefix `./subdir`: Resolve `subdir` relative to the current directory.
- Prefix `https://example.com/docs/`: Use the full HTTPS URL with the fragment.
- Prefix with spaces: Percent-encode the space before URI resolution.
- Prefix `#`: Use the fragment identifier (hash-only URIs).
- Prefix `/absolute/path`: Use absolute path with resolved segments.
- Prefix `./a/../b`: Resolve dot segments per RFC 3986.

### 3.7 TOC Output

The implementation must follow these TOC output rules:

- Emit `- [TITLE](DESTINATION)` for each selected heading.
- Escape link-label characters so rendering reproduces the canonical title.
- Use two spaces for each nesting level.
- Nest a heading under the nearest prior selected heading with a lower heading number.
- Place a heading at the root when no such heading exists.
- Reset nesting at each source boundary.
- Preserve source and heading order.
- Add one line ending after each item.
- Emit zero bytes for an empty standalone TOC.

## 4. Insertion, Writing, and Errors

### 4.1 Marker Recognition

A marker must satisfy these requirements:

- The marker must occupy the complete line.
- The marker must be an HTML comment: `<!-- MARK -->`.
- `MARK` must be nonempty text.
- `MARK` must not include leading whitespace, trailing whitespace, line endings, `<`, `>`, or `--`.

The implementation must ignore matching text inside fenced or indented code blocks.

### 4.2 Insertion Algorithm

The implementation must follow these insertion rules:

- Use the first eligible marker as the opening marker.
- Use the next eligible matching marker as the closing marker.
- Replace every byte between a paired marker set.
- Preserve both existing marker lines.
- Leave later marker pairs unchanged.
- Insert the TOC after a lone opening marker.
- Add a canonical closing marker after that TOC.
- Preserve all original content after the new closing marker.
- Return an error when no opening marker exists.
- Name the path and expected marker in that error.
- Tell the user to add the marker or select another value with `--mark`.
- Preserve every byte outside the replaced range.
- Use the opening marker's line-ending style for generated lines.
- Use LF when the opening marker has no line ending.
- Keep delimiters on consecutive lines when the TOC is empty.

### 4.3 Output Destinations

The implementation must follow these destination rules:

- Write only the TOC to stdout during normal operation.
- Write the complete modified document to stdout when `--insert` lacks `--write`.
- Write the complete modified document to the insertion path when `--write` is used.
- Produce no stdout content during successful `--write`.

### 4.4 Output Ordering

The implementation must finish all reads and validation before the first content write.

### 4.5 File Writing

The implementation must follow these file writing rules:

- The implementation must perform in-place writes through an atomic same-directory replacement.
- The implementation must preserve the target file mode.
- The implementation must resolve a symbolic-link target before replacement. The implementation must preserve the symbolic link.
- The implementation must keep the original file unchanged when replacement fails.

### 4.6 Process Behavior

The implementation must follow these process behavior rules:

- Return zero for success and help.
- Return two for command syntax and flag validation errors.
- Return one for input, parsing, insertion, and output errors.
- Write diagnostics only to stderr.
- Write no content after an error detected before destination writing.
- Exempt help and diagnostics from the normal content-only output rule.
- Emit no progress messages or logs.

## 5. Acceptance Test Cases

Test these representative cases to verify implementation compliance:

### 5.1 Input Sources

- Stdin without explicit insertion.
- Implicit insertion sources (insertion path without positional arguments).
- Multiple files with preserved order.
- Duplicate paths parsed each time.

### 5.2 Headings

- Formatted headings (bold, italics, links).
- Unicode headings.
- Empty headings.
- Duplicate headings (fragment collision).

### 5.3 Level Selection

- Heading-level jumps (H1 to H3).
- Source-boundary nesting reset.
- Minimum and maximum level boundaries.

### 5.4 Filtering

- Inclusion filters combined with OR.
- Exclusion filters rejecting matches.
- Mixed inclusion and exclusion filters.
- Regex patterns containing `/`.

### 5.5 Link Construction

- Relative prefixes (`.`, `./subdir`).
- Absolute path prefixes (`/path/to/docs`).
- HTTPS prefixes (`https://example.com/`).
- URI-reserved characters in paths.
- Paths with spaces and special characters.

### 5.6 Markers and Insertion

- Paired markers with existing content.
- Lone opening markers (no closing marker).
- Missing markers (no opening marker).
- Repeated marker pairs.
- Markers inside code blocks (ignored).
- LF and CRLF line endings.

### 5.7 Output

- Empty TOCs (zero bytes).
- TOC-only output to stdout.
- Complete modified document to stdout.
- In-place file replacement with `--write`.

### 5.8 Errors and Exit Codes

- Help output (exit zero).
- Flag syntax errors (exit two).
- Missing input files (exit one).
- Invalid marker text (exit two).
- UTF-8 encoding errors (exit one).
- File write failures (exit one).

## 6. Document Standards

### 6.1 Markdown and Validation

When a Markdown validator is available, users must run Markdown validation on this document. Users must report unavailable checks accurately.

### 6.2 ASD-STE100 Compliance

This document follows ASD-STE100. All requirements use:

- Short sentences (one instruction per sentence).
- Active voice.
- Consistent terminology.
- Numbered lists for procedures.
- Clear rejection criteria.

### 6.3 Requirement Completeness

Each requirement has one authoritative location. Acceptance review must confirm:

- Every original requirement is addressed.
- Every resolved issue is documented.
- No ambiguity remains in observable behavior.
- Validation, errors, and acceptance cases are specified.
