// Package hexdiff 提供 HexForge 的文件版本行级差异计算与重放能力。
// 数据仅在内存中处理，由上层负责版本的持久化。
//
// 行以 LF 结束，结束符属于该行；CRLF 中的 CR 属于行内容。
// 空文件为 0 行；文件末尾的 LF 不额外产生空行；没有结束符的末行仍算一行。
// 不做任何空白、换行或 Unicode 规范化。
package hexdiff

import (
	"errors"
	"fmt"
	"strings"
)

// Op 表示差异操作类型。
type Op int

const (
	// Keep 保留旧版中的一行，该行在新旧两版中逐字节相同。
	Keep Op = iota
	// Delete 删除旧版中的一行。
	Delete
	// Insert 插入新版中的一行。
	Insert
)

// Edit 表示一条行级差异操作。
// Keep/Delete 使用 OldIndex 引用旧版行；Insert 使用 NewIndex 引用新版行，
// 并携带该行的完整内容（含行结束符，若有）。
type Edit struct {
	Op       Op
	OldIndex int    // Keep/Delete 有效：旧版中的行号（从 0 开始）
	NewIndex int    // Keep/Insert 有效：新版中的行号（从 0 开始）
	Line     string // Insert 有效：插入行的完整内容
}

// Diff 是两个版本之间的可重放行级差异。
type Diff struct {
	Edits   []Edit
	Added   int // 新增行数（Insert 操作数）
	Deleted int // 删除行数（Delete 操作数）
}

// Compare 比较旧版与新版原文，返回可重放的行级差异。
// 返回的差异保证 Added+Deleted 达到全局最小值，且同一输入稳定产生同一结果。
func Compare(oldText, newText string) Diff {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	edits := diffLines(oldLines, newLines)
	d := Diff{Edits: edits}
	for _, e := range edits {
		switch e.Op {
		case Insert:
			d.Added++
		case Delete:
			d.Deleted++
		}
	}
	return d
}

// Replay 根据旧版原文与差异重放出新版内容。
// 差异合法（由 Compare 产生）时，结果与 Compare 时的新版原文逐字节一致。
func Replay(oldText string, d Diff) (string, error) {
	oldLines := splitLines(oldText)
	var sb strings.Builder
	cursor := 0 // 下一条 Keep/Delete 期望的旧版行号
	for i, e := range d.Edits {
		switch e.Op {
		case Keep:
			if e.OldIndex != cursor {
				return "", fmt.Errorf("hexdiff: 第 %d 条操作 Keep 的旧版行号 %d 与期望 %d 不符", i, e.OldIndex, cursor)
			}
			sb.WriteString(oldLines[cursor])
			cursor++
		case Delete:
			if e.OldIndex != cursor {
				return "", fmt.Errorf("hexdiff: 第 %d 条操作 Delete 的旧版行号 %d 与期望 %d 不符", i, e.OldIndex, cursor)
			}
			cursor++
		case Insert:
			sb.WriteString(e.Line)
		default:
			return "", fmt.Errorf("hexdiff: 第 %d 条操作类型非法: %d", i, e.Op)
		}
	}
	if cursor != len(oldLines) {
		return "", errors.New("hexdiff: 差异未覆盖旧版全部行")
	}
	return sb.String(), nil
}
