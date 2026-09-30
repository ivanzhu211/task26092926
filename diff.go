// Package hexfv computes byte-exact, in-memory, line-level differences
// between two versions of a file.
//
// The package exposes two operations: Compare produces an optimal (minimum
// insert+delete) line-level edit script, and Replay applies such a script to
// the original text to recover the revised text byte-for-byte. All data is
// held in memory; no disk or network resources are used.
package hexfv

import "errors"

// Maximum supported input size for a single version.
const (
	MaxBytes = 10 << 20 // 10 MiB
	MaxLines = 100_000
)

// ErrTooLarge is returned by Compare when an input exceeds MaxBytes or
// MaxLines.
var ErrTooLarge = errors.New("hexfv: input exceeds 10 MiB or 100000 lines")

// ErrInvalidScript is returned by Replay when ops do not consume the old
// text exactly or contain an unknown operation kind.
var ErrInvalidScript = errors.New("hexfv: invalid edit script")

// OpKind identifies how a single line is handled by an edit script.
type OpKind uint8

const (
	// OpKeep retains a line unchanged; the line exists in both versions.
	OpKeep OpKind = iota
	// OpDelete removes a line present in the old version.
	OpDelete
	// OpInsert adds a line present only in the new version.
	OpInsert
)

// Op is one entry of a line-level edit script. Content holds exactly one
// line including its terminator (LF, or CRLF as part of the content); the
// final line may have no terminator.
type Op struct {
	Kind    OpKind
	Content []byte
}

// Stats summarizes the size of a comparison.
type Stats struct {
	OldLines int
	NewLines int
	Inserted int
	Deleted  int
}

// Diff is the result of Compare.
type Diff struct {
	Ops   []Op
	Stats Stats
}

// Compare returns a minimum-size (Inserted+Deleted is globally minimal)
// edit script transforming oldText into newText, along with edit counts.
// The result is deterministic for identical inputs.
func Compare(oldText, newText []byte) (Diff, error) {
	if len(oldText) > MaxBytes || len(newText) > MaxBytes ||
		countLines(oldText) > MaxLines || countLines(newText) > MaxLines {
		return Diff{}, ErrTooLarge
	}

	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	oldIDs, newIDs, _ := intern(oldLines, newLines)
	kinds := align(oldIDs, newIDs)

	ops := make([]Op, 0, len(kinds))
	oldPos, newPos := 0, 0
	inserted, deleted := 0, 0
	for _, kind := range kinds {
		switch kind {
		case OpKeep:
			ops = append(ops, Op{Kind: OpKeep, Content: oldLines[oldPos]})
			oldPos++
			newPos++
		case OpDelete:
			ops = append(ops, Op{Kind: OpDelete, Content: oldLines[oldPos]})
			oldPos++
			deleted++
		case OpInsert:
			ops = append(ops, Op{Kind: OpInsert, Content: newLines[newPos]})
			newPos++
			inserted++
		}
	}

	return Diff{
		Ops: ops,
		Stats: Stats{
			OldLines: len(oldLines),
			NewLines: len(newLines),
			Inserted: inserted,
			Deleted:  deleted,
		},
	}, nil
}

// Replay applies ops to oldText and returns the reconstructed new text.
func Replay(oldText []byte, ops []Op) ([]byte, error) {
	oldLines := splitLines(oldText)
	oldPos := 0

	total := 0
	for _, op := range ops {
		total += len(op.Content)
	}
	out := make([]byte, 0, total)

	for _, op := range ops {
		switch op.Kind {
		case OpKeep, OpDelete:
			if oldPos >= len(oldLines) {
				return nil, ErrInvalidScript
			}
			if op.Kind == OpKeep {
				out = append(out, oldLines[oldPos]...)
			}
			oldPos++
		case OpInsert:
			out = append(out, op.Content...)
		default:
			return nil, ErrInvalidScript
		}
	}
	if oldPos != len(oldLines) {
		return nil, ErrInvalidScript
	}
	return out, nil
}
