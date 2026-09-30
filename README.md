# HexForge 文件版本差异模块

Go 库 `hexdiff`，提供两个文本版本之间的**行级差异计算**与**重放**。
数据仅在内存中处理，版本的持久化由上层负责。

## 能力

```go
import "hexforge/hexdiff"

d := hexdiff.Compare(oldText, newText) // 返回 hexdiff.Diff
// d.Edits：Keep/Delete/Insert 操作序列（Insert 携带行内容）
// d.Added / d.Deleted：新增、删除行数

newText2, err := hexdiff.Replay(oldText, d) // 重放，与 newText 逐字节一致
```

## 语义约定

- 行以 LF 结束，结束符属于该行；CRLF 的 CR 属于行内容，LF 与 CRLF 不视为同行。
- 空文件为 0 行；末尾 LF 不额外产生空行；无结束符的末行仍算一行。
- 不做任何空白、换行、Unicode 规范化；Keep 行在两版中逐字节相同且保序。
- 替换与移动均表达为删除加插入。
- `Added + Deleted` 保证达到全局最小值（Myers O(ND) + 线性空间分治，
  非近似算法）；同一输入稳定产生同一结果。

## 规模与性能

- 支持每份文件最大 10 MiB / 100000 行。
- 规模测试（100000 行、最短增删总数 6000、重复行占比约 70%、改动散布全文件）
  实测：Compare 约 73 ms，进程峰值 RSS 约 30 MiB（Apple Silicon，Go 1.27）。
  运行 `go test ./hexdiff/ -run TestScale -v` 可复现。

## 测试

```sh
go test ./...          # 全部测试（含规模测试）
go test ./... -short   # 跳过规模测试
```

小规模用例（含重复代码、移动与修改混合）会与独立 DP 计算的最优值核对，
并逐字节验证重放；测试只校验增删总数等于最优值，接受任何同样最优的对齐。
