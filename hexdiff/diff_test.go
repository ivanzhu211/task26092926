package hexdiff

import (
	"math/rand"
	"strings"
	"testing"
)

// optimalDistance 用独立的全量 DP（行级 Levenshtein，Keep 代价 0，
// Delete/Insert 代价 1）计算最短增删总数，作为最优性的权威对照。
// 仅用于小规模测试，复杂度 O(len(a)*len(b))。
func optimalDistance(a, b []string) int {
	n, m := len(a), len(b)
	prev := make([]int, m+1)
	cur := make([]int, m+1)
	for j := 0; j <= m; j++ {
		prev[j] = j
	}
	for i := 1; i <= n; i++ {
		cur[0] = i
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1]
			} else {
				cur[j] = prev[j] + 1 // 删除 a[i-1]
				if cur[j-1]+1 < cur[j] {
					cur[j] = cur[j-1] + 1 // 插入 b[j-1]
				}
			}
		}
		prev, cur = cur, prev
	}
	return prev[m]
}

// checkDiff 验证差异的基本性质：统计一致、重放逐字节一致、
// 增删总数等于独立计算的最优值。
func checkDiff(t *testing.T, oldText, newText string) Diff {
	t.Helper()
	d := Compare(oldText, newText)
	added, deleted := 0, 0
	for _, e := range d.Edits {
		switch e.Op {
		case Insert:
			added++
		case Delete:
			deleted++
		}
	}
	if added != d.Added || deleted != d.Deleted {
		t.Fatalf("统计不一致: Edits 中 added=%d deleted=%d, Diff 报告 Added=%d Deleted=%d",
			added, deleted, d.Added, d.Deleted)
	}
	got, err := Replay(oldText, d)
	if err != nil {
		t.Fatalf("Replay 报错: %v", err)
	}
	if got != newText {
		t.Fatalf("重放结果不一致:\n旧版: %q\n新版: %q\n重放: %q", oldText, newText, got)
	}
	opt := optimalDistance(splitLines(oldText), splitLines(newText))
	if d.Added+d.Deleted != opt {
		t.Fatalf("增删总数非最优: 得到 %d, 最优 %d\n旧版: %q\n新版: %q",
			d.Added+d.Deleted, opt, oldText, newText)
	}
	// 确定性：同一输入必须产生同一结果。
	d2 := Compare(oldText, newText)
	if len(d.Edits) != len(d2.Edits) {
		t.Fatalf("同一输入两次比较结果长度不同: %d vs %d", len(d.Edits), len(d2.Edits))
	}
	for i := range d.Edits {
		if d.Edits[i] != d2.Edits[i] {
			t.Fatalf("同一输入两次比较结果在第 %d 条不同: %+v vs %+v", i, d.Edits[i], d2.Edits[i])
		}
	}
	return d
}

func TestCompareFixedCases(t *testing.T) {
	cases := []struct {
		name    string
		oldText string
		newText string
	}{
		{"双空文件", "", ""},
		{"空到非空", "", "a\nb\n"},
		{"非空到空", "a\nb\n", ""},
		{"完全相同", "a\nb\nc\n", "a\nb\nc\n"},
		{"完全相同无末行结束符", "a\nb", "a\nb"},
		{"单行修改", "a\nb\nc\n", "a\nx\nc\n"},
		{"末尾新增无结束符", "a\nb\n", "a\nb\nc"},
		{"末行获得结束符", "a\nb", "a\nb\n"},
		{"末行失去结束符", "a\nb\n", "a\nb"},
		{"混合换行LF改CRLF", "a\nb\n", "a\r\nb\n"},
		{"混合换行CRLF改LF", "a\r\nb\r\n", "a\nb\r\n"},
		{"LF与CRLF不视为同行", "x\n", "x\r\n"},
		{"空行插入", "a\nb\n", "a\n\n\nb\n"},
		{"整体移动", "a\nb\nc\nd\n", "c\nd\na\nb\n"},
		{"移动加修改", "f1\nf2\nf3\nf4\n", "f3\nf4\nf1\nf2x\n"},
		{"重复行块", "x\nx\nx\nx\n", "x\nx\n"},
		{"重复行块扩增", "x\nx\n", "x\nx\nx\nx\nx\n"},
		{"Unicode行", "héllo\n世界\n", "héllo\n世界!\n"},
		{"相似函数", "func a() {\n\treturn 1\n}\nfunc b() {\n\treturn 2\n}\n",
			"func a() {\n\treturn 1\n}\nfunc b() {\n\treturn 3\n}\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkDiff(t, c.oldText, c.newText)
		})
	}
}

// TestCompareRandom 用高重复度的小行表随机生成用例，
// 覆盖重复行、重复块、移动与修改混合的场景，
// 每个用例都与独立 DP 的最优值核对并验证逐字节重放。
func TestCompareRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(20261001))
	// 行表刻意小且含空行、括号行，使重复行占比很高。
	vocab := []string{
		"\n", "{\n", "}\n", ")\n", "(\n",
		"func f() {\n", "return 0\n", "return 1\n",
		"a\n", "b\n", "x\n",
	}
	randLines := func(n int) []string {
		lines := make([]string, n)
		for i := range lines {
			lines[i] = vocab[rng.Intn(len(vocab))]
		}
		return lines
	}
	for trial := 0; trial < 2000; trial++ {
		n := rng.Intn(12)
		a := randLines(n)
		var b []string
		switch rng.Intn(4) {
		case 0: // 随机重写
			b = randLines(rng.Intn(12))
		case 1: // 复制后做零散增删改
			b = append([]string(nil), a...)
			for i := 0; i < len(b); i++ {
				if rng.Intn(3) == 0 {
					b[i] = vocab[rng.Intn(len(vocab))]
				}
			}
			if rng.Intn(2) == 0 && len(b) > 0 {
				b = append(b, vocab[rng.Intn(len(vocab))])
			}
		case 2: // 移动一个块并可能修改
			b = append([]string(nil), a...)
			if len(b) >= 2 {
				i := rng.Intn(len(b))
				j := rng.Intn(len(b))
				if i > j {
					i, j = j, i
				}
				block := append([]string(nil), b[i:j+1]...)
				b = append(b[:i], b[j+1:]...)
				pos := rng.Intn(len(b) + 1)
				b = append(b[:pos], append(block, b[pos:]...)...)
				if rng.Intn(2) == 0 && len(b) > 0 {
					b[rng.Intn(len(b))] = vocab[rng.Intn(len(vocab))]
				}
			}
		default: // 前缀+新中段+后缀
			b = append([]string(nil), a...)
			mid := randLines(rng.Intn(5))
			pos := rng.Intn(len(b) + 1)
			b = append(b[:pos], append(mid, b[pos:]...)...)
		}
		oldText := strings.Join(a, "")
		newText := strings.Join(b, "")
		checkDiff(t, oldText, newText)
	}
}

// TestReplayInvalid 验证重放对非法差异给出错误而非静默产出错误结果。
func TestReplayInvalid(t *testing.T) {
	if _, err := Replay("a\n", Diff{Edits: []Edit{{Op: Keep, OldIndex: 1}}}); err == nil {
		t.Fatal("行号乱序应报错")
	}
	if _, err := Replay("a\nb\n", Diff{Edits: []Edit{{Op: Keep, OldIndex: 0}}}); err == nil {
		t.Fatal("未覆盖全部旧行应报错")
	}
	if _, err := Replay("a\n", Diff{Edits: []Edit{{Op: Op(99)}}}); err == nil {
		t.Fatal("非法操作类型应报错")
	}
}
