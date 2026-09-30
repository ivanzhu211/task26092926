package hexfv

// splitLines divides data into lines without normalization.
//
// The terminating LF belongs to its line; a CR preceding the LF is part of
// the line content, so LF- and CRLF-terminated lines never compare equal.
// An empty input has zero lines; a trailing LF does not produce an extra
// empty line; a final line without a terminator still counts as one line.
func splitLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	lines := make([][]byte, 0, countLines(data))
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i+1:i+1])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:len(data):len(data)])
	}
	return lines
}

// internTable maps byte-identical lines to compact integer ids.
type internTable struct {
	// dict maps a line to its dense id. Keys reference the input data
	// directly; no bytes are copied.
	dict map[string]int32
	// lines preserves the first occurrence of every distinct line; the
	// slice at each position is the canonical content for that id.
	lines [][]byte
}

// countLines reports how many lines splitLines would return.
func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}

// id returns the id of line, assigning a fresh id on first sight.
func (t *internTable) id(line []byte) int32 {
	key := string(line)
	if id, ok := t.dict[key]; ok {
		return id
	}
	id := int32(len(t.lines))
	t.dict[key] = id
	t.lines = append(t.lines, line)
	return id
}

// intern assigns one shared id per byte-identical line across both versions.
func intern(oldLines, newLines [][]byte) (oldIDs, newIDs []int32, table *internTable) {
	table = &internTable{dict: make(map[string]int32, len(oldLines)+len(newLines))}
	oldIDs = make([]int32, len(oldLines))
	newIDs = make([]int32, len(newLines))
	for i, line := range oldLines {
		oldIDs[i] = table.id(line)
	}
	for i, line := range newLines {
		newIDs[i] = table.id(line)
	}
	return oldIDs, newIDs, table
}
