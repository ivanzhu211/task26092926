package hexfv

import (
	"bytes"
	"errors"
	"math/rand"
	"testing"
)

// checkRoundTrip compares old vs want, verifies counts against an
// independently computed optimum, validates script structure, and replays
// byte-for-byte.
func checkRoundTrip(t *testing.T, old, want []byte) {
	t.Helper()
	d, err := Compare(old, want)
	if err != nil {
		t.Fatalf("Compare error: %v", err)
	}

	oldIDs, newIDs, _ := intern(splitLines(old), splitLines(want))
	optDist := bruteDistance(oldIDs, newIDs)
	if got := d.Stats.Inserted + d.Stats.Deleted; got != optDist {
		t.Fatalf("edit distance = %d, want optimal %d", got, optDist)
	}

	// Re-derived counts from ops must match Stats.
	ins, del, keeps := 0, 0, 0
	for _, op := range d.Ops {
		switch op.Kind {
		case OpInsert:
			ins++
		case OpDelete:
			del++
		case OpKeep:
			keeps++
		}
	}
	if ins != d.Stats.Inserted || del != d.Stats.Deleted {
		t.Fatalf("counts mismatch: ops (%d ins/%d del) vs stats (%d/%d)",
			ins, del, d.Stats.Inserted, d.Stats.Deleted)
	}
	if keeps != d.Stats.OldLines-d.Stats.Deleted {
		t.Fatalf("keep count inconsistent")
	}

	// Ops carrying actual content, not just statistics.
	for _, op := range d.Ops {
		if len(op.Content) == 0 {
			t.Fatal("op carries empty content")
		}
	}

	got, err := Replay(old, d.Ops)
	if err != nil {
		t.Fatalf("Replay error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("replay mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestEdgeCases(t *testing.T) {
	checkRoundTrip(t, []byte(""), []byte(""))
	checkRoundTrip(t, []byte(""), []byte("only\nnew\nlines\n"))
	checkRoundTrip(t, []byte("only\nold\nlines\n"), []byte(""))
	checkRoundTrip(t, []byte("no newline at end"), []byte("changed without end too"))
	checkRoundTrip(t, []byte("a\nb"), []byte("a\nb\n"))
	checkRoundTrip(t, []byte("a\nb\n"), []byte("a\nb"))
}

func TestMixedEndingsAreDifferent(t *testing.T) {
	// Identical visible text but different terminators must not be treated
	// as identical lines; replay must preserve every CR byte.
	old := []byte("line\r\nkeep\nline\ntail")
	new := []byte("line\nkeep\nline\r\ntail")
	checkRoundTrip(t, old, new)

	d, _ := Compare(old, new)
	// All four lines differ between the versions: "line\r\n" vs "line\n"
	// are distinct content, and "tail" vs "tail\r" are distinct content.
	if d.Stats.Deleted+d.Stats.Inserted != 4 {
		t.Fatalf("CRLF swap distance = %d, want 4", d.Stats.Deleted+d.Stats.Inserted)
	}
}

func TestDuplicateFunctionsAndMoves(t *testing.T) {
	// A synthetic "source file": many duplicated braces/blank lines, two
	// near-identical functions (a repeated block), a move, and an inline
	// modification. A naive differ tends to rewrite whole blocks; the
	// optimum is much smaller.
	old := []byte(
		"func a() {\n" +
			"\treturn 1\n" +
			"}\n" +
			"\n" +
			"func b() {\n" +
			"\treturn 1\n" +
			"}\n" +
			"\n" +
			"func c() {\n" +
			"\treturn 3\n" +
			"}\n")
	new := []byte(
		// c moved to the front.
		"func c() {\n" +
			"\treturn 3\n" +
			"}\n" +
			"\n" +
			"func a() {\n" +
			"\treturn 1\n" +
			"}\n" +
			"\n" +
			// b's body changed by one line.
			"func b() {\n" +
			"\treturn 2\n" +
			"}\n")
	checkRoundTrip(t, old, new)

	d, _ := Compare(old, new)
	// Moving the 4-line block (signature+body+close+blank) of c to the
	// front is a delete+insert in line terms, and one body line is replaced
	// (delete+insert): at most 4+4+2 = 10. The independent optimum is
	// enforced by checkRoundTrip regardless of alignment.
	if d.Stats.Inserted+d.Stats.Deleted > 10 {
		t.Fatalf("distance %d exceeds expected small optimum", d.Stats.Inserted+d.Stats.Deleted)
	}
}

func TestDeterministicPublicAPI(t *testing.T) {
	old := []byte("x\ny\nz\nx\ny\n")
	new := []byte("y\nx\nz\ny\nx\n")
	first, err := Compare(old, new)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		again, err := Compare(old, new)
		if err != nil {
			t.Fatal(err)
		}
		if len(again.Ops) != len(first.Ops) {
			t.Fatal("inconsistent op count")
		}
		for j := range again.Ops {
			if again.Ops[j].Kind != first.Ops[j].Kind ||
				!bytes.Equal(again.Ops[j].Content, first.Ops[j].Content) {
				t.Fatalf("nondeterministic op at %d", j)
			}
		}
	}
}

func TestTooLarge(t *testing.T) {
	big := bytes.Repeat([]byte("x\n"), MaxLines+1)
	if _, err := Compare(big, big); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	bigBytes := bytes.Repeat([]byte("x"), MaxBytes+1)
	if _, err := Compare(bigBytes, bigBytes); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestReplayInvalidScript(t *testing.T) {
	old := []byte("a\nb\n")
	d, _ := Compare(old, []byte("a\nc\n"))

	// Script claims to keep two lines but forgets the delete: it consumes
	// only two of the three old lines.
	short := []Op{
		{Kind: OpDelete, Content: []byte("a\n")},
		{Kind: OpInsert, Content: []byte("c\n")},
	}
	if _, err := Replay(old, short); !errors.Is(err, ErrInvalidScript) {
		t.Fatalf("incomplete script err = %v", err)
	}
	// Extra trailing op over-consumes the old text.
	extra := append(append([]Op{}, d.Ops...), Op{Kind: OpDelete, Content: []byte("z\n")})
	if _, err := Replay(old, extra); !errors.Is(err, ErrInvalidScript) {
		t.Fatalf("over-consumed script err = %v", err)
	}
	// Unknown kind.
	bad := []Op{{Kind: OpKeep, Content: []byte("a\n")}, {Kind: 42, Content: []byte("?")}}
	if _, err := Replay(old, bad); !errors.Is(err, ErrInvalidScript) {
		t.Fatalf("bad kind err = %v", err)
	}
}

func TestUnchangedFileProducesOnlyKeeps(t *testing.T) {
	text := []byte("a\nb\nc\n")
	d, err := Compare(text, text)
	if err != nil {
		t.Fatal(err)
	}
	if d.Stats.Inserted != 0 || d.Stats.Deleted != 0 {
		t.Fatal("identical inputs must produce zero edits")
	}
	for _, op := range d.Ops {
		if op.Kind != OpKeep {
			t.Fatal("identical inputs must produce only keeps")
		}
	}
	checkRoundTrip(t, text, text)
}

func TestTextFuzzRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	// Pool mixes LF-terminated, CRLF-terminated and non-terminated-looking
	// tokens; short and duplicated, to stress ambiguity resolution.
	pool := []string{
		"func f() {\n",
		"func f() {\r\n",
		"\n",
		"\r\n",
		"}\n",
		"}\r\n",
		"    return x\n",
		"x\n",
		"中文字面量\n",
		"tail-no-newline",
	}
	for iter := 0; iter < 300; iter++ {
		gen := func(n int) []byte {
			var b bytes.Buffer
			for i := 0; i < n; i++ {
				b.WriteString(pool[rng.Intn(len(pool))])
			}
			return b.Bytes()
		}
		old := gen(rng.Intn(25))
		new := gen(rng.Intn(25))
		checkRoundTrip(t, old, new)
	}
}
