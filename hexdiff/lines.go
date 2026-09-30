package hexdiff

// splitLines 将文本切分为行。每行包含其 LF 结束符；
// 末行若无 LF 仍算一行；空文本返回 0 行。不做任何规范化。
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	n := 1
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			n++
		}
	}
	if s[len(s)-1] == '\n' {
		n--
	}
	lines := make([]string, 0, n)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
