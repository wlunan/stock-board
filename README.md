# Stock Board

轻量级 Windows 桌面股票/行情悬浮看板，纯 Go + Win32 API 实现，单文件 exe，无依赖。

## 功能

- **多品种支持** — A 股指数/个股、ETF、国际期货/贵金属、人民币金价
- **实时刷新** — 自动更新行情（默认 5 秒，可在配置中调整）
- **置顶悬浮** — 始终在最上层，不干扰其他操作
- **透明度可调** — 右键菜单 +/- 调节，最低几乎不可见
- **窗口可缩放** — 拖拽边缘或右键菜单放大/缩小
- **涨跌颜色** — 支持涨红跌绿 / 涨绿跌红切换
- **配置热加载** — 修改 `config.json` 后自动生效，无需重启
- **双击打开行情页** — 双击任意行情行，自动打开对应股票/期货网页
- **网络异常提示** — API 请求失败时显示红色"网络异常"提示
- **窗口位置记忆** — 退出时保存位置和大小，下次打开恢复
- **零依赖** — 单个 exe，放到任意目录即可运行

## 快速开始

### 直接使用

下载 `stock-board.exe`，放到任意目录，双击运行。程序会在同目录下自动生成 `config.json`。

### 从源码构建

```bash
# 前提：安装 Go 1.21+
# https://go.dev/dl/

git clone https://github.com/lunan/stock-board.git
cd stock-board
go mod tidy
go build -ldflags="-s -w -H windowsgui" -o stock-board.exe
```

编译参数说明：

| 参数 | 作用 |
|------|------|
| `-s` | 去掉符号表，减小体积 |
| `-w` | 去掉调试信息，减小体积 |
| `-H windowsgui` | Windows GUI 子系统，不弹控制台窗口 |

## 配置

程序启动时读取 **exe 同目录** 下的 `config.json`，不存在则自动生成默认配置。运行期间修改此文件会自动热加载。

### 完整字段

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `stocks` | `[]object` | 见下方 | 品种列表，每项含 `code` 和 `name` |
| `top_most` | `bool` | `true` | 窗口置顶 |
| `opacity` | `int` | `220` | 透明度 0-255（0=全透明，255=不透明） |
| `pos_x` | `int` | `100` | 窗口 X 坐标（像素，退出时自动保存） |
| `pos_y` | `int` | `100` | 窗口 Y 坐标（像素，退出时自动保存） |
| `width` | `int` | 自动 | 窗口宽度（像素，首次按股票数计算） |
| `height` | `int` | 自动 | 窗口高度（像素，首次按股票数计算） |
| `color_mode` | `string` | `"red_up"` | 涨跌颜色：`"red_up"`（涨红跌绿）/ `"green_up"`（涨绿跌红） |
| `refresh_interval` | `int` | `5` | 自动刷新间隔（秒），最小 1 |
| `font_size` | `int` | `18` | 字体大小（像素），范围 10-48 |

### 示例

```json
{
  "stocks": [
    {"code": "sh000001", "name": "上证指数"},
    {"code": "sz399001", "name": "深证成指"},
    {"code": "sh000300", "name": "沪深300"},
    {"code": "sz159995", "name": "芯片ETF"},
    {"code": "hf_XAU",   "name": "伦敦金"},
    {"code": "hf_SI",    "name": "纽约白银"},
    {"code": "gold_rmb", "name": "黄金(人民币)"}
  ],
  "top_most": true,
  "opacity": 220,
  "color_mode": "red_up",
  "refresh_interval": 5
}
```

## 数据源

### 1. 新浪财经实时行情

```
GET https://hq.sinajs.cn/list={codes}
Header: Referer: https://finance.sina.com.cn
```

返回 GBK 编码文本，每行一个品种。

### 2. tmini 金价 API

```
GET https://tmini.net/api/gold-price?type=json
```

返回 JSON，取 `metals[0]` 的 `sell_price` 和 `today_price` 计算涨跌。

### 品种代码格式

| 类型 | 代码格式 | 示例 |
|------|----------|------|
| 上海 A 股/指数 | `sh` + 代码 | `sh000001`（上证指数）、`sh600519`（贵州茅台） |
| 深圳 A 股/指数 | `sz` + 代码 | `sz399001`（深证成指）、`sz000858`（五粮液） |
| 国际期货 | `hf_` + 品种代码 | `hf_XAU`（伦敦金）、`hf_SI`（纽约白银） |
| 人民币金价 | `gold_rmb` | 调用 tmini API，单位：元/克 |

> `fx_s`（外汇）接口字段格式不兼容，暂不支持。

## 右键菜单

| 菜单项 | 功能 |
|--------|------|
| 置顶: 开/关 | 切换窗口置顶状态 |
| 透明度 +/- | 调节窗口透明度 |
| 放大 / 缩小 | 按比例缩放窗口 |
| 涨跌颜色 | 切换涨红跌绿 / 涨绿跌红 |
| 刷新 | 立即刷新行情 |
| 编辑股票... | 用记事本打开 `config.json` |
| 关于 | 版本和作者信息 |
| 退出 | 关闭程序 |

## 技术栈

- **语言**：Go 1.21
- **GUI**：Win32 API（`user32.dll`、`gdi32.dll`），无第三方 UI 框架
- **网络**：标准库 `net/http`
- **打包**：单文件 exe，约 5MB

## 待办 

- [x] 双击行情行打开对应网页
- [x] 行情异常/网络错误状态提示
- [ ] 复制行情到剪贴板
- [ ] 开机自启动选项
- [ ] 自定义刷新间隔
- [ ] 自定义字体大小

## License

MIT
