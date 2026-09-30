package hexdiff

import (
	"reflect"
	"testing"
)

func TestSplitLines(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"空文件", "", nil},
		{"仅一个LF", "\n", []string{"\n"}},
		{"单行无结束符", "abc", []string{"abc"}},
		{"单行有结束符", "abc\n", []string{"abc\n"}},
		{"末尾LF不产生空行", "a\nb\n", []string{"a\n", "b\n"}},
		{"末行无结束符", "a\nb", []string{"a\n", "b"}},
		{"中间空行", "a\n\nb\n", []string{"a\n", "\n", "b\n"}},
		{"连续空行", "\n\n\n", []string{"\n", "\n", "\n"}},
		{"CRLF整行含CR", "a\r\nb\r\n", []string{"a\r\n", "b\r\n"}},
		{"混合换行", "a\r\nb\nc", []string{"a\r\n", "b\n", "c"}},
		{"孤立CR", "a\rb\n", []string{"a\rb\n"}},
		{"Unicode原样", "héllo\n世界\n", []string{"héllo\n", "世界\n"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := splitLines(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("splitLines(%q) = %q,len=%d, want %q", c.in, got, len(got), c.want)
			}
		})
	}
}
