package hexfv

import (
	"bytes"
	"testing"
)

// FuzzCompare drives Compare+Replay on arbitrary byte inputs, checking
// byte-exact replay and the independent DP optimum for small sizes.
//
// Run with: go test -fuzz=FuzzCompare -fuzztime=30s
func FuzzCompare(f *testing.F) {
	seeds := [][2][]byte{
		{nil, nil},
		{[]byte("a\n"), []byte("a\r\n")},
		{[]byte("a\nb"), []byte("b\na\n")},
		{[]byte("\n\n\n"), []byte("\n")},
		{[]byte("x"), []byte("y")},
	}
	for _, s := range seeds {
		f.Add(s[0], s[1])
	}

	f.Fuzz(func(t *testing.T, old, new []byte) {
		if len(old) > 4096 || len(new) > 4096 ||
			countLines(old) > 200 || countLines(new) > 200 {
			return
		}
		d, err := Compare(old, new)
		if err != nil {
			if err != ErrTooLarge {
				t.Fatal(err)
			}
			return
		}
		got := d.Stats.Inserted + d.Stats.Deleted
		if got < 0 {
			t.Fatal("negative edit distance")
		}

		oldIDs, newIDs, _ := intern(splitLines(old), splitLines(new))
		if want := bruteDistance(oldIDs, newIDs); got != want {
			t.Fatalf("distance %d != optimum %d", got, want)
		}

		out, err := Replay(old, d.Ops)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out, new) {
			t.Fatalf("replay mismatch: got %q want %q", out, new)
		}
	})
}
