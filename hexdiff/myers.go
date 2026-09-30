package hexdiff

// diffLines 计算 a、b 两个行序列之间的最小编辑脚本（仅 Keep/Delete/Insert）。
// 使用 Myers O(ND) 算法及其线性空间分治改进，保证 Delete+Insert 总数
// 全局最小；平局时按固定规则选择，结果确定。
func diffLines(a, b []string) []Edit {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	ai, bi := internLines(a, b)
	d := differ{a: ai, b: bi, bs: b}
	d.diff(0, len(ai), 0, len(bi))
	return d.edits
}

// internLines 将行内容映射为整数 id，相同内容的行（跨 a、b）获得相同 id，
// 使核心算法的行比较退化为整数比较。
func internLines(a, b []string) ([]int32, []int32) {
	ids := make(map[string]int32, len(a)+len(b))
	intern := func(lines []string) []int32 {
		out := make([]int32, len(lines))
		for i, s := range lines {
			id, ok := ids[s]
			if !ok {
				id = int32(len(ids))
				ids[s] = id
			}
			out[i] = id
		}
		return out
	}
	return intern(a), intern(b)
}

type differ struct {
	a, b  []int32
	bs    []string // 新版行内容，用于 Insert 记录
	edits []Edit
}

func (d *differ) keep(ai, bi int) {
	d.edits = append(d.edits, Edit{Op: Keep, OldIndex: ai, NewIndex: bi})
}

func (d *differ) delete(ai int) {
	d.edits = append(d.edits, Edit{Op: Delete, OldIndex: ai})
}

func (d *differ) insert(bi int) {
	d.edits = append(d.edits, Edit{Op: Insert, NewIndex: bi, Line: d.bs[bi]})
}

// diff 递归计算 a[aLo:aHi] 与 b[bLo:bHi] 之间的最小编辑脚本并按序追加。
func (d *differ) diff(aLo, aHi, bLo, bHi int) {
	for aLo < aHi && bLo < bHi && d.a[aLo] == d.b[bLo] {
		d.keep(aLo, bLo)
		aLo++
		bLo++
	}
	suffix := 0
	for suffix < aHi-aLo && suffix < bHi-bLo &&
		d.a[aHi-1-suffix] == d.b[bHi-1-suffix] {
		suffix++
	}
	aMid, bMid := aHi-suffix, bHi-suffix
	switch n, m := aMid-aLo, bMid-bLo; {
	case n == 0:
		for i := bLo; i < bMid; i++ {
			d.insert(i)
		}
	case m == 0:
		for i := aLo; i < aMid; i++ {
			d.delete(i)
		}
	default:
		x, y := bisect(d.a[aLo:aMid], d.b[bLo:bMid])
		d.diff(aLo, aLo+x, bLo, bLo+y)
		d.diff(aLo+x, aMid, bLo+y, bMid)
	}
	for i := 0; i < suffix; i++ {
		d.keep(aMid+i, bMid+i)
	}
}

// bisect 在 a、b（调用前已剥离公共前后缀）之间寻找某条最优编辑路径上的
// 一个中间点 (x, y)。该点将问题一分为二，两侧各自最优即全局最优。
// 移植自 Myers 线性空间改进的经典实现（同 diff-match-patch 的 bisect）。
func bisect(a, b []int32) (int, int) {
	n, m := len(a), len(b)
	maxD := (n + m + 1) / 2
	vOffset := maxD
	vLen := 2*maxD + 2
	v1 := make([]int32, vLen)
	v2 := make([]int32, vLen)
	for i := range v1 {
		v1[i] = -1
		v2[i] = -1
	}
	v1[vOffset+1] = 0
	v2[vOffset+1] = 0
	delta := n - m
	front := delta%2 != 0
	k1start, k1end := 0, 0
	k2start, k2end := 0, 0
	nb, mb := int32(n), int32(m)
	for d := 0; d <= maxD; d++ {
		// 前向路径推进一步。
		for k1 := -d + k1start; k1 <= d-k1end; k1 += 2 {
			off := vOffset + k1
			var x1 int32
			if k1 == -d || (k1 != d && v1[off-1] < v1[off+1]) {
				x1 = v1[off+1]
			} else {
				x1 = v1[off-1] + 1
			}
			y1 := x1 - int32(k1)
			for x1 < nb && y1 < mb && a[x1] == b[y1] {
				x1++
				y1++
			}
			v1[off] = x1
			if x1 > nb {
				k1end += 2
			} else if y1 > mb {
				k1start += 2
			} else if front {
				k2off := vOffset + delta - k1
				if k2off >= 0 && k2off < vLen && v2[k2off] != -1 {
					if x2 := nb - v2[k2off]; x1 >= x2 {
						return int(x1), int(y1)
					}
				}
			}
		}
		// 反向路径推进一步。
		for k2 := -d + k2start; k2 <= d-k2end; k2 += 2 {
			off := vOffset + k2
			var x2 int32
			if k2 == -d || (k2 != d && v2[off-1] < v2[off+1]) {
				x2 = v2[off+1]
			} else {
				x2 = v2[off-1] + 1
			}
			y2 := x2 - int32(k2)
			for x2 < nb && y2 < mb && a[nb-1-x2] == b[mb-1-y2] {
				x2++
				y2++
			}
			v2[off] = x2
			if x2 > nb {
				k2end += 2
			} else if y2 > mb {
				k2start += 2
			} else if !front {
				k1off := vOffset + delta - k2
				if k1off >= 0 && k1off < vLen && v1[k1off] != -1 {
					x1 := v1[k1off]
					y1 := x1 - int32(delta-k2)
					if x2 = nb - x2; x1 >= x2 {
						return int(x1), int(y1)
					}
				}
			}
		}
	}
	// 不可达：a、b 非空且已剥离公共前后缀时，循环内必然相遇。
	return n, m
}
