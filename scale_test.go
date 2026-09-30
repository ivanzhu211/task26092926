package hexfv

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// buildUniqueReplacementFixture constructs exactly 100000 lines per side.
//
// 3000 replace sites are spread evenly across the file. At every site the
// old line is globally unique and the new line is a different globally
// unique value; all remaining 97000 "filler" lines are also globally
// unique. Therefore no changed line can be matched in either direction,
// every site forces one deletion plus one insertion, and the shortest
// edit distance is provably 3000*2 = 6000.
func buildUniqueReplacementFixture() (old, new []byte) {
	const total = 100_000
	const sites = 3_000
	siteSet := spreadSites(total, sites)

	var oldBuf, newBuf bytes.Buffer
	oldBuf.Grow(total * 16)
	newBuf.Grow(total * 16)
	for line := 0; line < total; line++ {
		if siteSet[line] {
			fmt.Fprintf(&oldBuf, "old-site-%d-x\n", line)
			fmt.Fprintf(&newBuf, "new-site-%d-y\n", line)
		} else {
			fmt.Fprintf(&oldBuf, "filler-%d\n", line)
			fmt.Fprintf(&newBuf, "filler-%d\n", line)
		}
	}
	return oldBuf.Bytes(), newBuf.Bytes()
}

// buildDuplicateFixture constructs 100000 lines per side where more than
// 90% of lines belong to a pool of only 10 repeated values (so identical
// lines massively outnumber unique ones), while 3000 deletions and 3000
// insertions of globally unique lines are interleaved across the whole
// file. The unique old-only and new-only lines cannot match anything, so
// the distance is provably exactly 6000.
func buildDuplicateFixture() (old, new []byte) {
	const total = 100_000
	const sites = 3_000
	siteSet := spreadSites(total, sites)

	var oldBuf, newBuf bytes.Buffer
	oldBuf.Grow(total * 8)
	newBuf.Grow(total * 8)
	for line := 0; line < total; line++ {
		// Every site removes a unique old line and adds a unique new line.
		// All other positions emit one of 10 repeated filler lines, shared
		// at the same positions on both sides.
		if siteSet[line] {
			fmt.Fprintf(&oldBuf, "old-del-%d\n", line)
			fmt.Fprintf(&newBuf, "new-ins-%d\n", line)
		} else {
			filler := []byte("dup-line-0\n")
			filler[9] = byte('0' + line%10)
			oldBuf.Write(filler)
			newBuf.Write(filler)
		}
	}
	return oldBuf.Bytes(), newBuf.Bytes()
}

// buildClusteredFixture packs all 3000 replacements into one central run,
// the shape most hostile to divide-and-conquer splitting.
func buildClusteredFixture() (old, new []byte) {
	const total = 100_000
	const sites = 3_000
	start := (total - sites) / 2

	var oldBuf, newBuf bytes.Buffer
	oldBuf.Grow(total * 16)
	newBuf.Grow(total * 16)
	for line := 0; line < total; line++ {
		if line >= start && line < start+sites {
			fmt.Fprintf(&oldBuf, "old-site-%d-x\n", line)
			fmt.Fprintf(&newBuf, "new-site-%d-y\n", line)
		} else {
			fmt.Fprintf(&oldBuf, "filler-%d\n", line)
			fmt.Fprintf(&newBuf, "filler-%d\n", line)
		}
	}
	return oldBuf.Bytes(), newBuf.Bytes()
}

// buildMaxBytesFixture keeps 100000 lines but makes each line roughly 100
// bytes long, pushing each input close to the 10 MiB limit. Every line is
// unique; 3000 spread replacements give distance 6000.
func buildMaxBytesFixture() (old, new []byte) {
	const total = 100_000
	const sites = 3_000
	siteSet := spreadSites(total, sites)

	pad := bytes.Repeat([]byte("z"), 90)
	var oldBuf, newBuf bytes.Buffer
	oldBuf.Grow(total * 105)
	newBuf.Grow(total * 105)
	for line := 0; line < total; line++ {
		if siteSet[line] {
			fmt.Fprintf(&oldBuf, "old-%d-%sx\n", line, pad)
			fmt.Fprintf(&newBuf, "new-%d-%sy\n", line, pad)
		} else {
			fmt.Fprintf(&oldBuf, "keep-%d-%sk\n", line, pad)
			fmt.Fprintf(&newBuf, "keep-%d-%sk\n", line, pad)
		}
	}
	return oldBuf.Bytes(), newBuf.Bytes()
}

// spreadSites picks n deterministic, well-distributed positions out of
// total, without collisions.
func spreadSites(total, n int) map[int]bool {
	siteSet := make(map[int]bool, n)
	for i := 0; i < n; i++ {
		pos := (i*total)/n + (i % 7)
		for siteSet[pos] {
			pos = (pos + 1) % total
		}
		siteSet[pos] = true
	}
	return siteSet
}

// TestScale100k performs the required large-scale check. It is skipped
// unless HEXFV_SCALE=1, so the normal test suite stays fast.
//
// Reproduce with:
//
//	HEXFV_SCALE=1 go test -run TestScale100k -v -count=1 .
//
// Process-wide peak RSS is best measured externally:
//
//	go test -c -o /private/tmp/hexfv.test
//	/usr/bin/time -l /private/tmp/hexfv.test -test.run TestScale100k -test.v
func TestScale100k(t *testing.T) {
	if os.Getenv("HEXFV_SCALE") != "1" {
		t.Skip("set HEXFV_SCALE=1 to run the 100000-line scale test")
	}

	fixtures := []struct {
		name    string
		builder func() ([]byte, []byte)
	}{
		{"unique_replacement", buildUniqueReplacementFixture},
		{"duplicate_heavy", buildDuplicateFixture},
		{"clustered_replacements", buildClusteredFixture},
		{"max_bytes_long_lines", buildMaxBytesFixture},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			old, new := fixture.builder()
			if len(bytes.Split(old, []byte{'\n'}))-1 != 100_000 {
				t.Fatal("fixture must contain 100000 lines")
			}

			runtime.GC()
			var base runtime.MemStats
			runtime.ReadMemStats(&base)

			start := time.Now()
			d, err := Compare(old, new)
			compareElapsed := time.Since(start)
			if err != nil {
				t.Fatalf("Compare error: %v", err)
			}

			var peak runtime.MemStats
			runtime.ReadMemStats(&peak)

			startReplay := time.Now()
			got, err := Replay(old, d.Ops)
			replayElapsed := time.Since(startReplay)
			if err != nil {
				t.Fatalf("Replay error: %v", err)
			}
			if !bytes.Equal(got, new) {
				t.Fatal("scale replay is not byte-identical")
			}

			if d.Stats.OldLines != 100_000 || d.Stats.NewLines != 100_000 {
				t.Fatalf("line stats = %+v", d.Stats)
			}
			if d.Stats.Inserted != 3_000 || d.Stats.Deleted != 3_000 {
				t.Fatalf("stats = %+v, want 3000 ins / 3000 del", d.Stats)
			}
			if d.Stats.Inserted+d.Stats.Deleted != 6_000 {
				t.Fatalf("distance = %d, want 6000", d.Stats.Inserted+d.Stats.Deleted)
			}

			heapPeak := int64(peak.TotalAlloc - base.TotalAlloc)
			t.Logf("input: old=%d bytes new=%d bytes", len(old), len(new))
			t.Logf("ops=%d inserted=%d deleted=%d",
				len(d.Ops), d.Stats.Inserted, d.Stats.Deleted)
			t.Logf("Compare: %v  Replay: %v",
				compareElapsed.Round(time.Millisecond),
				replayElapsed.Round(time.Millisecond))
			t.Logf("heap bytes allocated during Compare: %s",
				humanBytes(heapPeak))
			t.Logf("in-use heap after Compare: %s",
				humanBytes(int64(peak.HeapInuse)))
			t.Logf("Go runtime peak RSS (Sys): %s",
				humanBytes(int64(peak.Sys)))
			if compareElapsed > 10*time.Second {
				t.Fatalf("Compare took %v, want <= 10s", compareElapsed)
			}
		})
	}
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f MiB", float64(n)/float64(div))
}
