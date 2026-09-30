package hexfv

import (
	"bytes"
	"testing"
)

func TestSplitLines(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"lf only", "\n", []string{"\n"}},
		{"two lf", "\n\n", []string{"\n", "\n"}},
		{"trailing lf", "a\n", []string{"a\n"}},
		{"two lines", "a\nb\n", []string{"a\n", "b\n"}},
		{"no final newline", "a\nb", []string{"a\n", "b"}},
		{"crlf", "a\r\nb\r\n", []string{"a\r\n", "b\r\n"}},
		{"lone cr not terminator", "a\rb\n", []string{"a\rb\n"}},
		{"mixed endings", "a\r\nb\nc", []string{"a\r\n", "b\n", "c"}},
		{"empty line middle", "a\n\nb\n", []string{"a\n", "\n", "b\n"}},
		{"utf8 unchanged", "中\n文\n", []string{"中\n", "文\n"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitLines([]byte(tc.in))
			if len(got) != len(tc.want) {
				t.Fatalf("line count = %d, want %d (%q)", len(got), len(tc.want), tc.want)
			}
			for i, w := range tc.want {
				if !bytes.Equal(got[i], []byte(w)) {
					t.Fatalf("line %d = %q, want %q", i, got[i], w)
				}
			}
			if joined := bytes.Join(got, nil); !bytes.Equal(joined, []byte(tc.in)) {
				t.Fatalf("rejoin = %q, want %q", joined, tc.in)
			}
		})
	}
}

func TestInternDistinguishesCRLF(t *testing.T) {
	oldLines := splitLines([]byte("a\n"))
	newLines := splitLines([]byte("a\r\n"))
	oldIDs, newIDs, table := intern(oldLines, newLines)
	if oldIDs[0] == newIDs[0] {
		t.Fatal("LF and CRLF lines must intern to different ids")
	}
	if len(table.lines) != 2 {
		t.Fatalf("distinct lines = %d, want 2", len(table.lines))
	}

	oldLines2 := splitLines([]byte("x\ny\n"))
	newLines2 := splitLines([]byte("y\nx\n"))
	oldIDs2, newIDs2, table2 := intern(oldLines2, newLines2)
	if len(table2.lines) != 2 {
		t.Fatalf("distinct lines = %d, want 2", len(table2.lines))
	}
	if oldIDs2[0] != newIDs2[1] || oldIDs2[1] != newIDs2[0] {
		t.Fatal("identical lines must share ids regardless of position")
	}
}
