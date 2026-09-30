package hexfv

import (
	"math/rand"
	"testing"
)

// bruteDistance returns the minimum insert+delete count between a and b via
// an independent O(N*M) LCS dynamic program.
func bruteDistance(a, b []int32) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		cur[0] = 0
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
			} else if prev[j] >= cur[j-1] {
				cur[j] = prev[j]
			} else {
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
	}
	lcs := prev[len(b)]
	return len(a) + len(b) - 2*lcs
}

// validateScript executes ops against id sequences and reports validity and
// edit distance.
func validateScript(t *testing.T, a, b []int32, ops []OpKind) (ok bool, dist int) {
	t.Helper()
	i, j := 0, 0
	for _, op := range ops {
		switch op {
		case OpKeep:
			if i >= len(a) || j >= len(b) || a[i] != b[j] {
				t.Errorf("invalid keep at i=%d j=%d", i, j)
				return false, 0
			}
			i++
			j++
		case OpDelete:
			if i >= len(a) {
				t.Errorf("invalid delete at i=%d", i)
				return false, 0
			}
			i++
			dist++
		case OpInsert:
			if j >= len(b) {
				t.Errorf("invalid insert at j=%d", j)
				return false, 0
			}
			j++
			dist++
		}
	}
	if i != len(a) || j != len(b) {
		t.Errorf("script consumed (%d,%d) of (%d,%d)", i, j, len(a), len(b))
		return false, 0
	}
	return true, dist
}

func TestAlignHandCases(t *testing.T) {
	cases := []struct {
		name string
		a, b []int32
	}{
		{"both empty", nil, nil},
		{"only insert", nil, []int32{1, 2}},
		{"only delete", []int32{1, 2}, nil},
		{"equal", []int32{1, 2, 3}, []int32{1, 2, 3}},
		{"replace middle", []int32{1, 2, 3}, []int32{1, 9, 3}},
		{"move", []int32{1, 2, 3}, []int32{2, 3, 1}},
		{"all duplicate equal", []int32{7, 7, 7}, []int32{7, 7, 7}},
		{"duplicates differ count", []int32{7, 7}, []int32{7, 7, 7}},
		{"duplicates interleave", []int32{1, 7, 2}, []int32{7, 7, 1, 2}},
		{"crossing alternatives", []int32{1, 2, 1, 2}, []int32{2, 1, 2, 1}},
		{"classic abbc/baba", []int32{1, 2, 2, 3}, []int32{2, 1, 2, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := align(tc.a, tc.b)
			ok, got := validateScript(t, tc.a, tc.b, ops)
			want := bruteDistance(tc.a, tc.b)
			if ok && got != want {
				t.Fatalf("edit distance = %d, want optimal %d", got, want)
			}
		})
	}
}

func TestAlignDeterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	a := randomIDs(rng, 60, 5)
	b := randomIDs(rng, 60, 5)
	first := align(a, b)
	for i := 0; i < 5; i++ {
		again := align(a, b)
		if len(again) != len(first) {
			t.Fatal("inconsistent script length")
		}
		for k := range again {
			if again[k] != first[k] {
				t.Fatal("nondeterministic script")
			}
		}
	}
}

func randomIDs(rng *rand.Rand, n, alphabet int) []int32 {
	s := make([]int32, n)
	for i := range s {
		s[i] = int32(rng.Intn(alphabet))
	}
	return s
}

func TestAlignFuzzOptimal(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for iter := 0; iter < 2000; iter++ {
		n := rng.Intn(80)
		m := rng.Intn(80)
		// Occasionally use a single symbol (all lines identical) or a
		// large alphabet (near-unique lines).
		var alpha int
		switch rng.Intn(6) {
		case 0:
			alpha = 1
		case 5:
			alpha = 1000
		default:
			alpha = 1 + rng.Intn(5)
		}
		a := randomIDs(rng, n, alpha)
		b := randomIDs(rng, m, alpha)
		ops := align(a, b)
		ok, got := validateScript(t, a, b, ops)
		if !ok {
			t.Fatalf("invalid script for a=%v b=%v", a, b)
		}
		want := bruteDistance(a, b)
		if got != want {
			t.Fatalf("distance %d != optimal %d for a=%v b=%v", got, want, a, b)
		}
	}
}
