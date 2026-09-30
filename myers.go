package hexfv

// align computes a shortest edit script over two integer-id sequences.
// Equal ids denote byte-identical lines. The returned script has a globally
// minimal number of OpDelete+OpInsert entries; ties are resolved by the
// fixed Myers middle-snake search, so identical inputs always yield the
// same script.
//
// Implementation: Myers' O((N+M)*D) divide-and-conquer middle-snake
// algorithm ("An O(ND) Difference Algorithm and Its Variations", 1986),
// using linear working space per recursion level.
func align(oldIDs, newIDs []int32) []OpKind {
	ops := make([]OpKind, 0, len(oldIDs)+len(newIDs))

	// workspace holds per-recursion-depth reusable diagonals arrays, so
	// the divide-and-conquer search allocates O(N+M) ints in total rather
	// than one pair of arrays per internal node.
	ws := &workspace{
		vf: make([][]int, 0, 64),
		vb: make([][]int, 0, 64),
	}

	var rec func(x0, y0, x1, y1 int)
	rec = func(x0, y0, x1, y1 int) {
		// Consume and emit the common prefix in order.
		for x0 < x1 && y0 < y1 && oldIDs[x0] == newIDs[y0] {
			ops = append(ops, OpKeep)
			x0++
			y0++
		}

		// Locate the common suffix; emit it only after the middle region.
		common := 0
		for x1-common > x0 && y1-common > y0 &&
			oldIDs[x1-1-common] == newIDs[y1-1-common] {
			common++
		}
		mx1 := x1 - common
		my1 := y1 - common

		switch {
		case x0 == mx1 && y0 == my1:
			// Only a common suffix remains.
		case x0 == mx1:
			for y := y0; y < my1; y++ {
				ops = append(ops, OpInsert)
			}
		case y0 == my1:
			for x := x0; x < mx1; x++ {
				ops = append(ops, OpDelete)
			}
		default:
			vf, vb := ws.acquire(mx1 - x0 + my1 - y0)
			sx, sy, ex, ey := middleSnake(vf, vb, oldIDs, newIDs, x0, y0, mx1, my1)
			rec(x0, y0, sx, sy)
			for x, y := sx, sy; x < ex || y < ey; x, y = x+1, y+1 {
				ops = append(ops, OpKeep)
			}
			rec(ex, ey, mx1, my1)
			ws.release()
		}

		for i := 0; i < common; i++ {
			ops = append(ops, OpKeep)
		}
	}

	rec(0, 0, len(oldIDs), len(newIDs))
	return ops
}

type workspace struct {
	vf [][]int
	vb [][]int
}

// middleSnake finds a middle snake of the subproblem spanning
// oldIDs[x0:x1] and newIDs[y0:y1]. It returns the start (sx,sy) and end
// (ex,ey) of the snake in absolute coordinates. Both subproblem sizes are
// non-zero on entry.
func middleSnake(vf, vb []int, oldIDs, newIDs []int32, x0, y0, x1, y1 int) (sx, sy, ex, ey int) {
	n := x1 - x0
	m := y1 - y0
	delta := n - m
	size := n + m

	// Forward and reverse searches share identical transition rules. The
	// reverse search is the forward search on the mirrored subproblem:
	// mirrored coordinate (i,j) maps to original (n-i, m-j), with diagonal
	// k' = delta-k. No reversed arrays are materialized; mirrored cells are
	// addressed directly.
	//
	// Neighbor-trick seeds: diagonal +/-1 holds a free-edge endpoint.
	vf[size+1] = 0
	vb[size+1] = 0

	for d := 0; d <= size; d++ {
		// Forward furthest-reaching d-paths on the original frame.
		for k := -d; k <= d; k += 2 {
			ki := size + k
			var x int
			if k == -d || (k != d && vf[ki-1] < vf[ki+1]) {
				x = vf[ki+1] // down edge: insert
			} else {
				x = vf[ki-1] + 1 // right edge: delete
			}
			y := x - k
			u, v := x, y
			for u < n && v < m && oldIDs[x0+u] == newIDs[y0+v] {
				u++
				v++
			}
			vf[ki] = u

			// Overlap against the reverse (d-1)-paths.
			if delta%2 != 0 {
				kp := delta - k
				if kp >= -(d-1) && kp <= d-1 {
					if u >= n-vb[size+kp] {
						return x0 + x, y0 + y, x0 + u, y0 + v
					}
				}
			}
		}

		// Reverse furthest-reaching d-paths, computed as forward paths on
		// the mirrored frame.
		for kp := -d; kp <= d; kp += 2 {
			ki := size + kp
			var xp int
			if kp == -d || (kp != d && vb[ki-1] < vb[ki+1]) {
				xp = vb[ki+1] // mirrored down = original left edge
			} else {
				xp = vb[ki-1] + 1 // mirrored right = original up edge
			}
			yp := xp - kp
			up, vp := xp, yp
			for up < n && vp < m &&
				oldIDs[x1-1-up] == newIDs[y1-1-vp] {
				up++
				vp++
			}
			vb[ki] = up

			// Overlap against the forward d-paths.
			if delta%2 == 0 {
				k := delta - kp
				if k >= -d && k <= d {
					if vf[size+k]+up >= n {
						// Mirrored snake [xp,yp]..[up,vp] maps to the
						// original middle snake (n-up,m-vp)..(n-xp,m-yp).
						return x0 + n - up, y0 + m - vp, x0 + n - xp, y0 + m - yp
					}
				}
			}
		}
	}
	// Unreachable for non-empty inputs: an edit path always exists.
	panic("hexfv: middle snake not found")
}

// acquire returns a pair of zeroed diagonals arrays long enough for a
// subproblem whose combined side length is size. Arrays are indexed by
// recursion depth; a depth is only ever in use by one node at a time
// because both children are resolved before the parent releases.
func (ws *workspace) acquire(size int) (vf, vb []int) {
	need := 2*size + 3
	if len(ws.vf) == cap(ws.vf) {
		ws.vf = append(ws.vf, nil)
		ws.vb = append(ws.vb, nil)
	} else {
		ws.vf = ws.vf[:len(ws.vf)+1]
		ws.vb = ws.vb[:len(ws.vb)+1]
	}
	idx := len(ws.vf) - 1
	if cap(ws.vf[idx]) < need {
		ws.vf[idx] = make([]int, need)
		ws.vb[idx] = make([]int, need)
	} else {
		ws.vf[idx] = ws.vf[idx][:need]
		ws.vb[idx] = ws.vb[idx][:need]
		clear(ws.vf[idx])
		clear(ws.vb[idx])
	}
	return ws.vf[idx], ws.vb[idx]
}

// release detaches the depth-top arrays so they can be reused by the next
// sibling subtree; their backing storage is retained.
func (ws *workspace) release() {
	ws.vf = ws.vf[:len(ws.vf)-1]
	ws.vb = ws.vb[:len(ws.vb)-1]
}
