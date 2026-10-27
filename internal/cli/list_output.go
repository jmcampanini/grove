package cli

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jmcampanini/grove/internal/github"
	"github.com/spf13/cobra"
)

// defaultListLimit is how many records grove issue list and grove pr list
// return when neither --limit nor --all is selected.
const defaultListLimit = 20

// listFormat is the output format a list command writes.
type listFormat int

const (
	listFormatTable listFormat = iota
	listFormatFzf
	listFormatJSON
	listFormatJSONL
	listFormatCSV
)

// listFlags are the flags grove issue list and grove pr list share.
type listFlags struct {
	all        bool
	csv        bool
	cursor     string
	fzf        bool
	json       bool
	jsonl      bool
	limit      int
	noHeader   bool
	outputFile string
	overwrite  bool
}

func (f *listFlags) register(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.BoolVar(&f.fzf, "fzf", false, "Print tab-separated lines for fzf")
	flags.BoolVar(&f.json, "json", false, "Write one JSON object holding the records")
	flags.BoolVar(&f.jsonl, "jsonl", false, "Write one JSON object per record per line")
	flags.BoolVar(&f.csv, "csv", false, "Write CSV with a header line")
	flags.BoolVar(&f.noHeader, "no-header", false, "Omit the CSV header line (requires --csv)")
	flags.IntVar(&f.limit, "limit", defaultListLimit, "Return at most `n` records")
	flags.BoolVar(&f.all, "all", false, "Return every record")
	flags.StringVar(&f.cursor, "cursor", "", "Resume after the record `cursor` names")
	flags.StringVar(&f.outputFile, "output-file", "", "Write the output to `path` instead of stdout")
	flags.BoolVar(&f.overwrite, "overwrite", false, "Replace or create the --output-file path")
	cmd.MarkFlagsMutuallyExclusive("fzf", "json", "jsonl", "csv")
	cmd.MarkFlagsMutuallyExclusive("limit", "all")
}

// validate rejects flag combinations the mutually exclusive groups cannot
// express. It runs as part of the positional grammar, so a rejected
// invocation is a usage error and does no command work.
func (f *listFlags) validate(cmd *cobra.Command, _ []string) error {
	flags := cmd.Flags()
	switch {
	case flags.Changed("limit") && f.limit < 1:
		return fmt.Errorf("--limit must be at least 1, got %d", f.limit)
	case flags.Changed("cursor") && f.cursor == "":
		return errors.New("--cursor cannot be empty")
	case f.noHeader && !f.csv:
		return errors.New("--no-header requires --csv")
	case f.overwrite && !flags.Changed("output-file"):
		return errors.New("--overwrite requires --output-file")
	case flags.Changed("output-file") && f.outputFile == "":
		return errors.New("--output-file cannot be empty")
	case f.outputFile != "" && !f.json && !f.jsonl && !f.csv:
		return errors.New("--output-file requires --json, --jsonl, or --csv")
	}
	return nil
}

func (f *listFlags) format() listFormat {
	switch {
	case f.fzf:
		return listFormatFzf
	case f.json:
		return listFormatJSON
	case f.jsonl:
		return listFormatJSONL
	case f.csv:
		return listFormatCSV
	default:
		return listFormatTable
	}
}

func (f *listFlags) request() github.ListRequest {
	return github.ListRequest{After: f.cursor, All: f.all, Limit: f.limit}
}

// checkOutputFile fails before any network work when --output-file names a
// path the command would refuse to replace.
func (f *listFlags) checkOutputFile() error {
	if f.outputFile == "" {
		return nil
	}
	info, err := os.Lstat(f.outputFile)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("failed to check output file %q: %w", f.outputFile, err)
	case info.IsDir():
		return fmt.Errorf("output file %q is a directory", f.outputFile)
	case !f.overwrite:
		return fmt.Errorf("output file %q already exists; pass --overwrite to replace it", f.outputFile)
	}
	return nil
}

// listRecord is a record of the --json, --jsonl, and --csv output.
type listRecord interface {
	// csvRow returns the record's CSV cells in column order.
	csvRow() []string
}

// listResult is what one list run collected. When hasMore is set, the run
// stopped at its limit and nextCursor resumes after the last record.
type listResult[T listRecord] struct {
	hasMore    bool
	nextCursor string
	records    []T
}

// listDocument is the --json output. Its field order is the key order.
type listDocument[T listRecord] struct {
	Items      []T    `json:"items"`
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor,omitempty"`
	Complete   bool   `json:"complete"`
}

// writeListPayload writes the records in the machine-readable format flags
// selects, to stdout or to the --output-file path.
func writeListPayload[T listRecord](cmd *cobra.Command, flags listFlags, result listResult[T], columns []string) error {
	write := func(w io.Writer) error {
		switch flags.format() {
		case listFormatJSON:
			return encodeListJSON(w, result)
		case listFormatJSONL:
			return encodeListJSONL(w, result.records)
		case listFormatCSV:
			return encodeListCSV(w, result.records, columns, flags.noHeader)
		default:
			return fmt.Errorf("format %d is not a machine-readable list format", flags.format())
		}
	}
	if flags.outputFile == "" {
		return write(cmd.OutOrStdout())
	}
	return writeOutputFile(flags.outputFile, flags.overwrite, write)
}

// reportMoreRecords tells the caller on stderr that the run stopped at its
// limit, for every format but --json, which carries hasMore and nextCursor.
func reportMoreRecords[T listRecord](cmd *cobra.Command, flags listFlags, noun string, result listResult[T]) error {
	if !result.hasMore || flags.format() == listFormatJSON {
		return nil
	}
	_, err := fmt.Fprintf(cmd.ErrOrStderr(), "more %s exist; continue with --cursor %s\n", noun, result.nextCursor)
	return err
}

// newJSONEncoder returns an encoder that leaves <, >, and & unescaped, so
// titles read the same as on GitHub.
func newJSONEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}

func encodeListJSON[T listRecord](w io.Writer, result listResult[T]) error {
	items := result.records
	if items == nil {
		items = []T{}
	}
	doc := listDocument[T]{
		Complete:   true,
		HasMore:    result.hasMore,
		Items:      items,
		NextCursor: result.nextCursor,
	}
	if err := newJSONEncoder(w).Encode(doc); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

func encodeListJSONL[T listRecord](w io.Writer, records []T) error {
	enc := newJSONEncoder(w)
	for _, record := range records {
		if err := enc.Encode(record); err != nil {
			return fmt.Errorf("failed to encode JSON line: %w", err)
		}
	}
	return nil
}

func encodeListCSV[T listRecord](w io.Writer, records []T, columns []string, noHeader bool) error {
	cw := csv.NewWriter(w)
	if !noHeader {
		if err := cw.Write(columns); err != nil {
			return fmt.Errorf("failed to write CSV: %w", err)
		}
	}
	for _, record := range records {
		if err := cw.Write(record.csvRow()); err != nil {
			return fmt.Errorf("failed to write CSV: %w", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("failed to write CSV: %w", err)
	}
	return nil
}

// compactJSONCell renders a nested value as compact JSON for one CSV cell.
func compactJSONCell(v any) string {
	var buf bytes.Buffer
	if err := newJSONEncoder(&buf).Encode(v); err != nil {
		// The callers pass string slices, which always encode.
		panic(fmt.Sprintf("encode CSV cell: %v", err))
	}
	return string(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
}

// optionalCell renders a value that may be absent as a CSV cell, empty when
// absent.
func optionalCell(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// writeOutputFile writes the payload under a temporary name next to path
// and moves it into place only after write, flush, sync, and close all
// succeed, so a failed run leaves path as it was. Without overwrite the move
// is a hard link, which fails when path already exists instead of replacing
// it.
func writeOutputFile(path string, overwrite bool, write func(io.Writer) error) error {
	tmp, err := createSiblingTemp(path)
	if err != nil {
		return fmt.Errorf("failed to write output file %q: %w", path, err)
	}
	tmpPath := tmp.Name()

	if err := writeAndClose(tmp, write); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write output file %q: %w", path, err)
	}

	if overwrite {
		if err := os.Rename(tmpPath, path); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("failed to write output file %q: %w", path, err)
		}
		return nil
	}

	err = os.Link(tmpPath, path)
	// The payload is in place once the link succeeds; a temporary name
	// left behind does not change it.
	_ = os.Remove(tmpPath)
	switch {
	case errors.Is(err, fs.ErrExist):
		return fmt.Errorf("output file %q already exists; pass --overwrite to replace it", path)
	case err != nil:
		return fmt.Errorf("failed to write output file %q: %w", path, err)
	}
	return nil
}

func writeAndClose(f *os.File, write func(io.Writer) error) error {
	buffered := bufio.NewWriter(f)
	err := write(buffered)
	if err == nil {
		err = buffered.Flush()
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// createSiblingTemp creates a new, empty file in the directory of path.
// Unlike os.CreateTemp it asks for mode 0666, so the umask decides the mode
// of the finished file as it would for any file the command created.
func createSiblingTemp(path string) (*os.File, error) {
	dir, base := filepath.Split(path)
	for range 10 {
		name := filepath.Join(dir, "."+base+".tmp-"+rand.Text())
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return f, err
	}
	return nil, errors.New("could not create a temporary file")
}

// Help text the list commands share. The record fields, the empty table
// message, and the noun differ per command and are composed around these.
const (
	helpListFormats = `--json, --jsonl, and --csv select a machine-readable format; choose at
most one of them or --fzf. They are never styled. --json writes one JSON
object, {"items": [...], "hasMore": ..., "nextCursor": ..., "complete":
true}: items holds the records, nextCursor is present only when hasMore
is true, and complete is always true because the command writes nothing
when a read fails. --jsonl writes one record per line as a JSON object
followed by a newline. --csv writes a header line naming the columns, then
one row per record; --no-header omits the header, so rows can be appended
to an existing file with >>. In CSV an empty or null value is an empty
cell and a list is one cell of compact JSON.`

	helpListOutputFile = `--output-file <path> writes the --json, --jsonl, or --csv output to path
instead of stdout. It refuses a path that already exists unless
--overwrite is selected, which replaces the file at path or creates it.
The output is written to a temporary file in the same directory and moved
to path only when complete, so a failed run leaves path unchanged.`
)

// helpListScope documents --limit, --all, and --cursor for a list command
// whose records are called noun, such as "issues".
func helpListScope(noun string) string {
	return fmt.Sprintf(`--limit <n> returns at most n records (default %d) and --all returns
every record; choose at most one. When the command stops at its limit
while GitHub has more records, --json sets hasMore to true and nextCursor
to a cursor, and every other format prints this line on stderr:

  more %[2]s exist; continue with --cursor <cursor>

The exit status stays 0. --cursor <cursor> resumes after the last record
of the run that reported it, and combines with --limit or --all. The
command reads GitHub in pages of up to 100 records, each a separate gh
command and cache entry, and writes nothing unless every page succeeds.`, defaultListLimit, noun)
}
