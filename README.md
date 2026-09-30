# hexfv — HexForge 文件版本差异库

纯内存、无外部依赖的 Go 库，提供两个版本之间的**行级差异计算**与
**按差异重放**。上层负责保存版本；本模块只负责比较与重放。

## 能力

- `Compare(oldText, newText []byte) (Diff, error)`
  - 返回行级编辑脚本（`OpKeep` / `OpDelete` / `OpInsert`）及
    `Stats{OldLines, NewLines, Inserted, Deleted}`。
  - `Inserted + Deleted` 是**所有合法差异中的全局最小值**
    （最长公共子序列 / 最短编辑脚本意义上），不是近似结果。
  - 相同输入稳定产生相同脚本。
  - 每个 `Op` 都带实际行内容；重放不需要新版原文。
- `Replay(oldText []byte, ops []Op) ([]byte, error)`
  - 仅用旧版原文与脚本重建新版，逐字节一致；脚本不合法时返回
    `ErrInvalidScript`。

## 行的定义（不做任何规范化）

- 行以 `\n` 结束，结束符属于该行；`\r\n` 中的 `\r` 也是行内容，
  因此 LF 与 CRLF 结尾**不是相同行**。
- 空文件为 0 行；末尾的 `\n` 不额外产生空行；没有结束符的末行
  仍算一行。空白、换行、Unicode 一律按原始字节处理。
- 替换、移动都表达为“删除 + 插入”；保留行必须在两份原文中逐字节
  相同且保持各自顺序。

## 算法

Myers 中蛇分治（divide-and-conquer middle snake），时间
`O((N+M)·D)`，额外空间线性。行先按字节内容归并为紧凑整数 id，
重复行只比较整数。递归各层的对角线索引数组通过 workspace 复用。

## 限制

- 单份输入最多 `MaxBytes = 10 MiB`、`MaxLines = 100000` 行；
  超出返回 `ErrTooLarge`。

## 使用

```go
d, err := hexfv.Compare(oldText, newText)
if err != nil { /* ErrTooLarge */ }
fmt.Println(d.Stats.Inserted, d.Stats.Deleted)
rebuilt, err := hexfv.Replay(oldText, d.Ops) // 与 newText 逐字节相同
```

## 测试与复现

```sh
# 常规单元测试：边界行规则、CRLF、空文件、无结束符末行、
# 重复函数块/移动/修改混合、确定性、非法脚本。
go test ./...

# 100000 行、最短增删总数恰为 6000 的规模测试（含重复行 >90%、
# 改动散布、改动聚集、接近 10 MiB 长行四种夹具），打印耗时与内存。
HEXFV_SCALE=1 go test -run TestScale100k -v -count=1 .

# 随机模糊：对拍独立 O(N*M) DP 最优值并校验逐字节重放。
go test -fuzz=FuzzCompare -fuzztime=30s
```

测试接受不同但同样最优的对齐：只校验脚本合法性、独立计算的最优
增删总数与逐字节重放，不把某一种排布当作唯一答案。
