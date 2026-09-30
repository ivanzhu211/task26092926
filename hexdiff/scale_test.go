package hexdiff

import (
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// maxRSS 返回进程峰值常驻内存（字节）。
// ru_maxrss 在 macOS 上单位为字节，在 Linux 上为 KB。
func maxRSS() int64 {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	if runtime.GOOS == "linux" {
		return ru.Maxrss * 1024
	}
	return ru.Maxrss
}

// TestScale 规模测试：100000 行、最短增删总数恰好 6000、重复行占比
// 超过 50%、改动散布全文件。验证最优性（增删总数恰为 6000，构造上已
// 证明 6000 即全局最优：LCS 上界为 97000，距离下界即 6000）、逐字节
// 重放，并输出可复现的耗时与峰值内存。
func TestScale(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过规模测试")
	}
	const total = 100000
	const dels = 3000
	const ins = 3000

	rng := rand.New(rand.NewSource(42))
	// 重复池使重复行占比远超 50%。
	pool := []string{"{\n", "}\n", "\n", ")\n", "return 0\n"}
	oldLines := make([]string, 0, total)
	for i := 0; i < total; i++ {
		if rng.Intn(10) < 7 {
			oldLines = append(oldLines, pool[rng.Intn(len(pool))])
		} else {
			oldLines = append(oldLines, fmt.Sprintf("line_%d\n", i))
		}
	}
	// 统计重复行占比，确保满足 >50% 的前提。
	seen := make(map[string]int, len(oldLines))
	for _, l := range oldLines {
		seen[l]++
	}
	dup := 0
	for _, l := range oldLines {
		if seen[l] > 1 {
			dup++
		}
	}
	if dup*2 <= total {
		t.Fatalf("重复行占比未超过 50%%: %d/%d", dup, total)
	}

	// 均匀散布地选择 3000 个删除位置与 3000 个插入位置。
	delIdx := make(map[int]bool, dels)
	perm := rng.Perm(total)
	for i := 0; i < dels; i++ {
		delIdx[perm[i]] = true
	}
	newLines := make([]string, 0, total)
	insPositions := rng.Perm(total + 1)[:ins]
	insAt := make(map[int][]string, ins)
	for _, p := range insPositions {
		insAt[p] = append(insAt[p], fmt.Sprintf("inserted_unique_%d\n", p))
	}
	for i := 0; i <= total; i++ {
		newLines = append(newLines, insAt[i]...)
		if i < total && !delIdx[i] {
			newLines = append(newLines, oldLines[i])
		}
	}
	oldText := strings.Join(oldLines, "")
	newText := strings.Join(newLines, "")
	t.Logf("旧版 %d 行 %d 字节, 新版 %d 行 %d 字节, 重复行占比 %.1f%%",
		len(oldLines), len(oldText), len(newLines), len(newText), float64(dup)*100/total)

	before := maxRSS()
	start := time.Now()
	d := Compare(oldText, newText)
	elapsed := time.Since(start)

	got, err := Replay(oldText, d)
	if err != nil {
		t.Fatalf("Replay 报错: %v", err)
	}
	if got != newText {
		t.Fatal("规模测试重放结果与新版不一致")
	}
	// 构造保证全局最优为 6000（见函数注释），必须恰好达到。
	if d.Added+d.Deleted != dels+ins {
		t.Fatalf("增删总数非最优: 得到 %d, 最优 %d", d.Added+d.Deleted, dels+ins)
	}
	if d.Added != ins || d.Deleted != dels {
		t.Fatalf("增删分类不符: Added=%d Deleted=%d, 期望 %d/%d", d.Added, d.Deleted, ins, dels)
	}

	peak := maxRSS()
	t.Logf("Compare 耗时: %v (上限 10s)", elapsed)
	t.Logf("进程峰值 RSS: %.1f MiB (上限 128 MiB), 测试前基线 %.1f MiB",
		float64(peak)/(1<<20), float64(before)/(1<<20))
	if elapsed > 10*time.Second {
		t.Errorf("耗时超限: %v > 10s", elapsed)
	}
	if peak > 128<<20 {
		t.Errorf("峰值内存超限: %.1f MiB > 128 MiB", float64(peak)/(1<<20))
	}
}
