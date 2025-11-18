# lucal (Go + Bubble Tea)

这是对传统 `lucal` 农历命令行工具的现代 TUI 重写版本。它保留了原有的使用语义，
同时添加了交互式的 Bubble Tea 界面作为默认体验。

![lucal](./screenshots/screenshot.0.png)

## 功能特性

- 默认开启交互式 TUI，支持流畅的键盘导航
- 非交互模式（`-n`），使用 Bubble Tea 的表格组件渲染传统风格的输出
- 精确的农历信息，由 [`github.com/Lofanmi/chinese-calendar-golang`](https://github.com/Lofanmi/chinese-calendar-golang) 提供支持
- 自动适应终端宽度的布局，年度视图每行最多显示 3 个月
- GBK 感知的宽度计算，确保中文文本与 ASCII 标签对齐整洁
- **节假日高亮**：中国法定节假日显示为蓝色，调休工作日显示为橙色
- **节假日数据自动更新**：使用 `-u` 标志下载最新节假日数据

## 系统要求

- Go 1.22+
- macOS/Linux 终端（Windows 可通过 WSL 运行）
- 年份范围在 **1900** 到 **3000** 之间（上游农历数据集的限制）

### 安装方式

#### Homebrew

```bash
brew install lululau/utils/lucal
```

#### 从源码安装

```bash
go install github.com/lululau/lucal/cmd/lucal@latest
```

## 构建与运行

```bash
git clone https://github.com/lululau/lucal.git
cd lucal
go build ./cmd/lucal
./lucal            # 显示当前月的交互式日历
./lucal -n -y 2024 # 一次性渲染 2024 年日历
```

### 命令行用法

```
lucal               # 当前月（交互式）
lucal -y            # 当前年
lucal 9             # 当年9月
lucal 1983          # 公元1983年
lucal 2012 12       # 2012年12月
lucal -y 9          # 公元9年的全年（受限于数据源，1900 年以前会报错）
lucal -n …          # 非交互模式，渲染输出后立即退出
lucal -u            # 下载最新的节假日数据
lucal -h <file>     # 指定节假日数据文件（用于调试）
```

### 交互式快捷键

| 按键        | 操作                           |
| ---------- | -------------------------------- |
| `k/[` / `j/]`  | 上一个月 / 下一个月            |
| `K/{` / `J/}`  | 上一年 / 下一年             |
| `.`        | 跳转回当前月份   |
| `y`        | 进入年份输入对话框          |
| `m`        | 进入月份输入对话框         |
| `q`        | 退出                             |
| `Esc`      | 取消当前输入对话框      |

提示：年份提示接受 `YYYY` 或 `YYYY MM` 格式，所以 `y` → `2025 10` 会立即跳转到 2025年10月。

### 节假日数据

日历可以高亮显示中国法定节假日和调休工作日：
- **节假日** 以 **蓝色** 显示
- **工作日**（调休）以 **橙色** 显示
- **今天** 以 **绿色** 显示（除非当天是节假日/工作日）

节假日数据会自动从 XDG 缓存目录（`~/.cache/lucal/holidays.json`）加载。
如果缓存不存在或超过 6 个月，日历底部会显示更新提醒。

更新节假日数据：
```bash
lucal -u
# 或者
lucal --update-holidays
```

这将从 GitHub 下载最新节假日数据并保存到缓存目录。
下载进度会通过进度条显示，包含速度和文件大小信息。

**节假日数据来源**：节假日信息来源于 [timor.tech API](https://timor.tech/api/holiday)，
该 API 提供中国法定节假日和调休工作日数据。

用于开发/调试，您可以指定自定义节假日数据文件：
```bash
lucal -h ./holidays.json
# 或者
lucal --holidays-file ./holidays.json
```

## 开发

```bash
go test ./...
go run ./cmd/lucal --help
```

开发期间，您可以以普通模式运行应用程序来检查布局变化：

```bash
go run ./cmd/lucal -n -y 2025
```

## 限制

- 农历数据源仅支持 1900–3000 年；更早的年份将显示明确的错误消息。
- 基于 GBK 的宽度检测在简体中文字体下效果最佳。混合 emoji/稀有字符会回退到 runewidth 近似值。

## 故障排除

- **年度视图太高**：将 `lucal -n -y <year>` 通过管道传送到分页器（如 `less -R`）以便更容易滚动。
- **颜色显示异常**：确保您的终端支持 24 位颜色。如有必要，请设置 `COLORTERM=truecolor`。