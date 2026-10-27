package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/grove/internal/issue"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListFlagsRejectInvalidInvocations(t *testing.T) {
	tests := []struct {
		args    []string
		name    string
		wantErr string
	}{
		{name: "two formats", args: []string{"--json", "--csv"}, wantErr: "none of the others can be"},
		{name: "json and jsonl", args: []string{"--json", "--jsonl"}, wantErr: "none of the others can be"},
		{name: "fzf and a format", args: []string{"--fzf", "--jsonl"}, wantErr: "none of the others can be"},
		{name: "limit and all", args: []string{"--limit", "5", "--all"}, wantErr: "none of the others can be"},
		{name: "overwrite without output file", args: []string{"--json", "--overwrite"}, wantErr: "--overwrite requires --output-file"},
		{name: "output file without a format", args: []string{"--output-file", "out.json"}, wantErr: "--output-file requires --json, --jsonl, or --csv"},
		{name: "output file with fzf", args: []string{"--fzf", "--output-file", "out.txt"}, wantErr: "--output-file requires --json, --jsonl, or --csv"},
		{name: "empty output file", args: []string{"--json", "--output-file", ""}, wantErr: "--output-file cannot be empty"},
		{name: "no header without csv", args: []string{"--json", "--no-header"}, wantErr: "--no-header requires --csv"},
		{name: "zero limit", args: []string{"--limit", "0"}, wantErr: "--limit must be at least 1, got 0"},
		{name: "negative limit", args: []string{"--limit=-3"}, wantErr: "--limit must be at least 1, got -3"},
		{name: "limit not a number", args: []string{"--limit", "ten"}, wantErr: `invalid argument "ten" for "--limit"`},
		{name: "empty cursor", args: []string{"--cursor", ""}, wantErr: "--cursor cannot be empty"},
		{name: "operand", args: []string{"extra"}, wantErr: `unknown command "extra"`},
	}

	for _, command := range [][]string{{"issue", "list"}, {"pr", "list"}} {
		for _, tt := range tests {
			t.Run(strings.Join(command, " ")+"/"+tt.name, func(t *testing.T) {
				t.Setenv("XDG_STATE_HOME", t.TempDir())
				var stdout, stderr bytes.Buffer
				args := append(append([]string{}, command...), tt.args...)
				err := executeWithFileLogging(strings.NewReader(""), &stdout, &stderr, args)

				require.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, exitUsage, exitStatus(err))
				assert.Empty(t, stdout.String())
			})
		}
	}
}

// TestListRecordColumnsMatchJSONKeys keeps the documented promise that the
// CSV columns are the JSON keys in field order.
func TestListRecordColumnsMatchJSONKeys(t *testing.T) {
	jsonKeys := func(record any) []string {
		var keys []string
		typ := reflect.TypeOf(record)
		for i := range typ.NumField() {
			keys = append(keys, typ.Field(i).Tag.Get("json"))
		}
		return keys
	}
	assert.Equal(t, jsonKeys(issueRecord{}), issueRecordColumns)
	assert.Equal(t, jsonKeys(prRecord{}), prRecordColumns)
	assert.Len(t, issueRecord{}.csvRow(), len(issueRecordColumns))
	assert.Len(t, prRecord{}.csvRow(), len(prRecordColumns))
}

func TestEncodeListCSVPullRequests(t *testing.T) {
	worktree := "/workspace/pr-42"
	records := []prRecord{
		{
			Author:            "alice",
			BaseBranch:        "main",
			Branch:            "feature/login",
			CreatedAt:         time.Date(2026, 7, 3, 9, 0, 0, 0, time.UTC),
			IsCrossRepository: true,
			Labels:            []string{"bug"},
			Number:            42,
			Title:             "Fix login",
			UpdatedAt:         time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
			URL:               "https://github.com/acme/app/pull/42",
			Worktree:          &worktree,
		},
		{
			BaseBranch: "release",
			Branch:     "docs",
			CreatedAt:  time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
			Labels:     []string{},
			Number:     40,
			Title:      "Docs",
			UpdatedAt:  time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC),
			URL:        "https://github.com/acme/app/pull/40",
		},
	}

	var buf bytes.Buffer
	require.NoError(t, encodeListCSV(&buf, records, prRecordColumns, false))

	assert.Equal(t, "number,title,author,branch,baseBranch,isCrossRepository,labels,createdAt,updatedAt,url,worktree\n"+
		`42,Fix login,alice,feature/login,main,true,"[""bug""]",2026-07-03T09:00:00Z,2026-07-04T09:00:00Z,https://github.com/acme/app/pull/42,/workspace/pr-42`+"\n"+
		`40,Docs,,docs,release,false,[],2026-07-01T09:00:00Z,2026-07-02T09:00:00Z,https://github.com/acme/app/pull/40,`+"\n",
		buf.String())
}

func TestIssueRecords(t *testing.T) {
	created := time.Date(2026, 7, 1, 9, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	match := issueMatch(17, "Fix <login> & crash", "alice", nil, "/workspace/is-17")
	match.Issue.CreatedAt = created
	match.Issue.URL = "https://github.com/acme/app/issues/17"
	records := issueRecords([]issue.Match{match, issueMatch(18, "Add logout", "bob", nil, "")})

	require.Len(t, records, 2)
	assert.Equal(t, created.UTC(), records[0].CreatedAt)
	require.NotNil(t, records[0].Worktree)
	assert.Equal(t, "/workspace/is-17", *records[0].Worktree)
	assert.Nil(t, records[1].Worktree)
	assert.Equal(t, []string{}, records[1].Labels)
}

func testIssueRecords() []issueRecord {
	worktree := "/workspace/is-17"
	created := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 7, 2, 10, 30, 0, 0, time.UTC)
	return []issueRecord{
		{
			Author:    "alice",
			CreatedAt: created,
			Labels:    []string{"bug", "ui, ux"},
			Number:    17,
			Title:     `Fix "login" <crash>`,
			UpdatedAt: updated,
			URL:       "https://github.com/acme/app/issues/17",
			Worktree:  &worktree,
		},
		{
			CreatedAt: created,
			Labels:    []string{},
			Number:    18,
			Title:     "Add logout",
			UpdatedAt: updated,
			URL:       "https://github.com/acme/app/issues/18",
		},
	}
}

func TestEncodeListJSON(t *testing.T) {
	tests := []struct {
		name   string
		result listResult[issueRecord]
		want   string
	}{
		{
			name:   "no records",
			result: listResult[issueRecord]{},
			want:   `{"items":[],"hasMore":false,"complete":true}` + "\n",
		},
		{
			name:   "stopped at the limit",
			result: listResult[issueRecord]{hasMore: true, nextCursor: "Y3Vyc29y", records: testIssueRecords()[1:]},
			want: `{"items":[{"number":18,"title":"Add logout","author":"","labels":[],` +
				`"createdAt":"2026-07-01T09:00:00Z","updatedAt":"2026-07-02T10:30:00Z",` +
				`"url":"https://github.com/acme/app/issues/18","worktree":null}],` +
				`"hasMore":true,"nextCursor":"Y3Vyc29y","complete":true}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, encodeListJSON(&buf, tt.result))
			assert.Equal(t, tt.want, buf.String())
			assert.True(t, json.Valid(buf.Bytes()))
		})
	}
}

func TestEncodeListJSONL(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, encodeListJSONL(&buf, testIssueRecords()))

	lines := strings.Split(buf.String(), "\n")
	require.Len(t, lines, 3, "two records, each followed by a newline")
	assert.Empty(t, lines[2])
	assert.Equal(t, `{"number":17,"title":"Fix \"login\" <crash>","author":"alice","labels":["bug","ui, ux"],`+
		`"createdAt":"2026-07-01T09:00:00Z","updatedAt":"2026-07-02T10:30:00Z",`+
		`"url":"https://github.com/acme/app/issues/17","worktree":"/workspace/is-17"}`, lines[0])
	for _, line := range lines[:2] {
		var object map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &object))
	}

	buf.Reset()
	require.NoError(t, encodeListJSONL(&buf, []issueRecord{}))
	assert.Empty(t, buf.String())
}

func TestEncodeListCSV(t *testing.T) {
	const header = "number,title,author,labels,createdAt,updatedAt,url,worktree\n"
	const rows = `17,"Fix ""login"" <crash>",alice,"[""bug"",""ui, ux""]",2026-07-01T09:00:00Z,2026-07-02T10:30:00Z,https://github.com/acme/app/issues/17,/workspace/is-17` + "\n" +
		`18,Add logout,,[],2026-07-01T09:00:00Z,2026-07-02T10:30:00Z,https://github.com/acme/app/issues/18,` + "\n"

	tests := []struct {
		name     string
		noHeader bool
		records  []issueRecord
		want     string
	}{
		{name: "header and rows", records: testIssueRecords(), want: header + rows},
		{name: "rows without header", noHeader: true, records: testIssueRecords(), want: rows},
		{name: "no records writes the header", records: nil, want: header},
		{name: "no records without header writes nothing", noHeader: true, records: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, encodeListCSV(&buf, tt.records, issueRecordColumns, tt.noHeader))
			assert.Equal(t, tt.want, buf.String())
		})
	}
}

func TestReportMoreRecords(t *testing.T) {
	more := listResult[issueRecord]{hasMore: true, nextCursor: "Y3Vyc29y"}
	tests := []struct {
		flags  listFlags
		name   string
		result listResult[issueRecord]
		want   string
	}{
		{name: "csv", flags: listFlags{csv: true}, result: more, want: "more issues exist; continue with --cursor Y3Vyc29y\n"},
		{name: "jsonl", flags: listFlags{jsonl: true}, result: more, want: "more issues exist; continue with --cursor Y3Vyc29y\n"},
		{name: "table", flags: listFlags{}, result: more, want: "more issues exist; continue with --cursor Y3Vyc29y\n"},
		{name: "json carries it in the payload", flags: listFlags{json: true}, result: more, want: ""},
		{name: "nothing more", flags: listFlags{csv: true}, result: listResult[issueRecord]{}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetErr(&stderr)
			require.NoError(t, reportMoreRecords(cmd, tt.flags, "issues", tt.result))
			assert.Equal(t, tt.want, stderr.String())
		})
	}
}

func TestWriteOutputFile(t *testing.T) {
	writeText := func(text string) func(io.Writer) error {
		return func(w io.Writer) error {
			_, err := io.WriteString(w, text)
			return err
		}
	}
	failAfter := func(text string) func(io.Writer) error {
		return func(w io.Writer) error {
			_, _ = io.WriteString(w, text)
			return errors.New("encode failed")
		}
	}

	t.Run("creates a missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.json")
		require.NoError(t, writeOutputFile(path, false, writeText("{}\n")))
		assertFileContent(t, path, "{}\n")
		assertNoTempFiles(t, filepath.Dir(path))
	})

	t.Run("overwrite creates a missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.json")
		require.NoError(t, writeOutputFile(path, true, writeText("{}\n")))
		assertFileContent(t, path, "{}\n")
	})

	t.Run("refuses an existing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.json")
		require.NoError(t, os.WriteFile(path, []byte("old\n"), 0o600))
		err := writeOutputFile(path, false, writeText("new\n"))
		require.ErrorContains(t, err, "already exists; pass --overwrite to replace it")
		assertFileContent(t, path, "old\n")
		assertNoTempFiles(t, filepath.Dir(path))
	})

	t.Run("overwrite replaces an existing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.json")
		require.NoError(t, os.WriteFile(path, []byte("old\n"), 0o600))
		require.NoError(t, writeOutputFile(path, true, writeText("new\n")))
		assertFileContent(t, path, "new\n")
		assertNoTempFiles(t, filepath.Dir(path))
	})

	t.Run("a failed write leaves an existing file unchanged", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.json")
		require.NoError(t, os.WriteFile(path, []byte("old\n"), 0o600))
		err := writeOutputFile(path, true, failAfter("partial"))
		require.ErrorContains(t, err, "encode failed")
		assertFileContent(t, path, "old\n")
		assertNoTempFiles(t, filepath.Dir(path))
	})

	t.Run("a failed write creates no file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.json")
		err := writeOutputFile(path, false, failAfter("partial"))
		require.ErrorContains(t, err, "encode failed")
		assert.NoFileExists(t, path)
		assertNoTempFiles(t, filepath.Dir(path))
	})

	t.Run("a missing directory fails", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing", "out.json")
		require.Error(t, writeOutputFile(path, true, writeText("{}\n")))
		assert.NoFileExists(t, path)
	})
}

func TestPublishOutputFileWithoutHardLinks(t *testing.T) {
	noLinks := func(string, string) error { return errors.ErrUnsupported }

	t.Run("renames into a missing path", func(t *testing.T) {
		dir := t.TempDir()
		tmp, path := filepath.Join(dir, ".out.json.tmp-x"), filepath.Join(dir, "out.json")
		require.NoError(t, os.WriteFile(tmp, []byte("new\n"), 0o600))

		require.NoError(t, publishOutputFile(tmp, path, false, noLinks))
		assertFileContent(t, path, "new\n")
		assert.NoFileExists(t, tmp)
	})

	t.Run("refuses a path that appeared since the check", func(t *testing.T) {
		dir := t.TempDir()
		tmp, path := filepath.Join(dir, ".out.json.tmp-x"), filepath.Join(dir, "out.json")
		require.NoError(t, os.WriteFile(tmp, []byte("new\n"), 0o600))
		require.NoError(t, os.WriteFile(path, []byte("old\n"), 0o600))

		err := publishOutputFile(tmp, path, false, noLinks)
		require.ErrorContains(t, err, "already exists; pass --overwrite to replace it")
		assertFileContent(t, path, "old\n")
	})
}

// failingFile is a syncFile whose writes, sync, or close fail.
type failingFile struct {
	closed   bool
	closeErr error
	syncErr  error
	writeErr error
	written  int
}

func (f *failingFile) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	f.written += len(p)
	return len(p), nil
}

func (f *failingFile) Sync() error { return f.syncErr }

func (f *failingFile) Close() error {
	f.closed = true
	return f.closeErr
}

func TestWriteAndCloseReportsEveryStep(t *testing.T) {
	failed := errors.New("disk full")
	writeSmall := func(w io.Writer) error {
		_, err := io.WriteString(w, "{}\n")
		return err
	}
	tests := []struct {
		file *failingFile
		name string
	}{
		{name: "flush", file: &failingFile{writeErr: failed}},
		{name: "sync", file: &failingFile{syncErr: failed}},
		{name: "close", file: &failingFile{closeErr: failed}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writeAndClose(tt.file, writeSmall)
			require.ErrorIs(t, err, failed)
			assert.True(t, tt.file.closed, "the file is closed after a failure")
		})
	}

	file := &failingFile{}
	require.NoError(t, writeAndClose(file, writeSmall))
	assert.Equal(t, 3, file.written, "the buffered payload is flushed")
}

func TestCheckOutputFile(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "out.json")
	require.NoError(t, os.WriteFile(existing, []byte("old\n"), 0o600))

	assert.NoError(t, (&listFlags{}).checkOutputFile())
	assert.NoError(t, (&listFlags{outputFile: filepath.Join(dir, "new.json")}).checkOutputFile())
	assert.NoError(t, (&listFlags{outputFile: existing, overwrite: true}).checkOutputFile())
	assert.ErrorContains(t, (&listFlags{outputFile: existing}).checkOutputFile(), "already exists; pass --overwrite to replace it")
	assert.ErrorContains(t, (&listFlags{outputFile: dir, overwrite: true}).checkOutputFile(), "is a directory")
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, want, string(data))
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".tmp-", "temporary file left behind")
	}
}
