package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")

	pCreateWindowExW     = user32.NewProc("CreateWindowExW")
	pDefWindowProcW      = user32.NewProc("DefWindowProcW")
	pDestroyWindow       = user32.NewProc("DestroyWindow")
	pDispatchMessageW    = user32.NewProc("DispatchMessageW")
	pGetMessageW         = user32.NewProc("GetMessageW")
	pPostQuitMessage     = user32.NewProc("PostQuitMessage")
	pRegisterClassExW    = user32.NewProc("RegisterClassExW")
	pShowWindow          = user32.NewProc("ShowWindow")
	pUpdateWindow        = user32.NewProc("UpdateWindow")
	pSetLayeredWindow    = user32.NewProc("SetLayeredWindowAttributes")
	pGetClientRect       = user32.NewProc("GetClientRect")
	pFillRect            = user32.NewProc("FillRect")
	pInvalidateRect      = user32.NewProc("InvalidateRect")
	pGetWindowRect       = user32.NewProc("GetWindowRect")
	pReleaseCapture      = user32.NewProc("ReleaseCapture")
	pSendMessageW        = user32.NewProc("SendMessageW")
	pPostMessageW        = user32.NewProc("PostMessageW")
	pBeginPaint          = user32.NewProc("BeginPaint")
	pEndPaint            = user32.NewProc("EndPaint")
	pCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	pAppendMenuW         = user32.NewProc("AppendMenuW")
	pDestroyMenu         = user32.NewProc("DestroyMenu")
	pGetCursorPos        = user32.NewProc("GetCursorPos")
	pTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pSetWindowLongW      = user32.NewProc("SetWindowLongW")
	pSetWindowPos        = user32.NewProc("SetWindowPos")
	pScreenToClient      = user32.NewProc("ScreenToClient")
	pExtTextOutW         = gdi32.NewProc("ExtTextOutW")
	pMessageBoxW         = user32.NewProc("MessageBoxW")
	pMultiByteToWideChar = kernel32.NewProc("MultiByteToWideChar")

	pSetBkMode        = gdi32.NewProc("SetBkMode")
	pSetTextColor     = gdi32.NewProc("SetTextColor")
	pCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	pCreateFontW      = gdi32.NewProc("CreateFontW")
	pSelectObject     = gdi32.NewProc("SelectObject")
	pDeleteObject     = gdi32.NewProc("DeleteObject")
)

const (
	WS_EX_LAYERED    = 0x00080000
	WS_EX_TOPMOST    = 0x00000008
	WS_EX_TOOLWINDOW = 0x00000080
	WS_POPUP         = 0x80000000
	WS_VISIBLE       = 0x10000000
	WS_THICKFRAME    = 0x00040000
	SW_SHOW          = 5
	LWA_ALPHA        = 0x00000002
	WM_PAINT         = 0x000F
	WM_NCCALCSIZE    = 0x0083
	WM_DESTROY       = 0x0002
	WM_EXITSIZEMOVE  = 0x0232
	WM_LBUTTONDOWN   = 0x0201
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONDOWN   = 0x0204
	WM_NCLBUTTONDOWN = 0x00A1
	WM_NCHITTEST     = 0x0084
	WM_SETCURSOR     = 0x0020
	WM_COMMAND       = 0x0111
	WM_USER          = 0x0400
	HTCAPTION        = 2
	HTLEFT           = 10
	HTRIGHT          = 11
	HTTOP            = 12
	HTTOPLEFT        = 13
	HTTOPRIGHT       = 14
	HTBOTTOM         = 15
	HTBOTTOMLEFT     = 16
	HTBOTTOMRIGHT    = 17
	TRANSPARENT      = 1
	FW_NORMAL        = 400
	TPM_RIGHTBUTTON  = 0x0002
	TPM_BOTTOMALIGN  = 0x0020
	MF_STRING        = 0x00000000
	MF_SEPARATOR     = 0x00000800
	WM_REFRESH_DONE  = WM_USER + 1
	MENU_TOPMOST     = 40001
	MENU_OPACITY_UP  = 40002
	MENU_OPACITY_DOWN= 40003
	MENU_REFRESH     = 40004
	MENU_SETTINGS    = 40005
	MENU_ABOUT       = 40006
	MENU_EXIT        = 40007
	MENU_SCALE_UP    = 40008
	MENU_SCALE_DOWN  = 40009
	MENU_COLOR_MODE = 40010
	SWP_NOSIZE      = 0x0001
	SWP_NOMOVE      = 0x0002
	SWP_NOZORDER    = 0x0004
	SWP_SHOWWINDOW  = 0x0040
	ETO_OPAQUE       = 0x0002
	CP936            = 936
)

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

type MSG struct {
	HWnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

type POINT struct {
	X, Y int32
}

type PAINTSTRUCT struct {
	Hdc         windows.Handle
	Erase       int32
	RcPaint     RECT
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

type StockItem struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// 黄金 API 响应结构
type GoldResponse struct {
	Metals []GoldMetal `json:"metals"`
}

type GoldMetal struct {
	Name      string `json:"name"`
	SellPrice string `json:"sell_price"`
	TodayPrice string `json:"today_price"`
}

type StockData struct {
	Name      string
	Price     float64
	Change    float64
	ChangePct float64
}

type Config struct {
	Stocks          []StockItem `json:"stocks"`
	TopMost         bool        `json:"top_most"`
	Opacity         int         `json:"opacity"`
	PosX            int         `json:"pos_x"`
	PosY            int         `json:"pos_y"`
	Width           int         `json:"width"`
	Height          int         `json:"height"`
	ColorMode       string      `json:"color_mode"`       // "red_up"=涨红跌绿(默认), "green_up"=涨绿跌红
	RefreshInterval int         `json:"refresh_interval"` // 刷新间隔（秒），默认 5
}

var (
	hwnd       windows.Handle
	stockData  []StockData
	stockMu    sync.RWMutex
	config     Config
	configPath string
	httpClient = &http.Client{Timeout: 8 * time.Second}
	refreshMu  sync.Mutex
	appDone    = make(chan struct{})
	stopOnce   sync.Once
	configMTime time.Time // config 文件最后修改时间，用于热加载
	lastFetchOK bool      // 最近一次行情获取是否成功
)

func u16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func u16s(s string) []uint16 {
	p, _ := syscall.UTF16FromString(s)
	return p
}

func drawStr(hdc uintptr, x, y int32, s string) {
	p := u16s(s)
	pExtTextOutW.Call(hdc, uintptr(x), uintptr(y), 0, 0, uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)-1), 0)
}

func rgb(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

func getConfigPath() string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "config.json")
}

func calcSize(stocks int) (int, int) {
	w := 310
	if stocks <= 3 {
		w = 310
	} else if stocks <= 6 {
		w = 320
	} else {
		w = 330
	}
	h := 10 + stocks*22 + 24
	return w, h
}

func defaultConfig() Config {
	stocks := []StockItem{
		{Code: "sh000001", Name: "上证指数"},
		{Code: "sz399001", Name: "深证成指"},
		{Code: "sh000300", Name: "沪深300"},
		{Code: "fx_susdcny", Name: "美元/人民币"},
		{Code: "fx_sgbpcny", Name: "英镑/人民币"},
	}
	w, h := calcSize(len(stocks))
	return Config{
		Stocks:          stocks,
		TopMost:         true,
		Opacity:         220,
		PosX:            100,
		PosY:            100,
		Width:           w,
		Height:          h,
		RefreshInterval: 5,
	}
}

func loadConfig() {
	configPath = getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		config = defaultConfig()
		saveConfig()
		return
	}
	if err := json.Unmarshal(data, &config); err != nil || len(config.Stocks) == 0 {
		config = defaultConfig()
		saveConfig()
		return
	}
	if config.Opacity == 0 {
		config.Opacity = 220
	}
	if config.ColorMode == "" {
		config.ColorMode = "red_up"
	}
	if config.Width == 0 || config.Height == 0 {
		config.Width, config.Height = calcSize(len(config.Stocks))
	}
	if config.RefreshInterval < 1 {
		config.RefreshInterval = 5
	}
	// 记录文件修改时间
	if fi, err := os.Stat(configPath); err == nil {
		configMTime = fi.ModTime()
	}
}

func saveConfig() {
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func decodeSinaBody(body []byte) string {
	if utf8.Valid(body) {
		return string(body)
	}
	if len(body) == 0 {
		return ""
	}
	n, _, _ := pMultiByteToWideChar.Call(CP936, 0, uintptr(unsafe.Pointer(&body[0])), uintptr(len(body)), 0, 0)
	if n == 0 {
		return string(body)
	}
	buf := make([]uint16, n)
	pMultiByteToWideChar.Call(CP936, 0, uintptr(unsafe.Pointer(&body[0])), uintptr(len(body)), uintptr(unsafe.Pointer(&buf[0])), n)
	return syscall.UTF16ToString(buf)
}

func fetchGoldPrice() StockData {
	var sd StockData
	resp, err := httpClient.Get("https://tmini.net/api/gold-price?type=json")
	if err != nil {
		return sd
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sd
	}
	var gr GoldResponse
	if err := json.Unmarshal(body, &gr); err != nil || len(gr.Metals) == 0 {
		return sd
	}
	// 取第一个品种（今日金价）
	m := gr.Metals[0]
	sellPrice, _ := strconv.ParseFloat(m.SellPrice, 64)
	todayPrice, _ := strconv.ParseFloat(m.TodayPrice, 64)
	pct := 0.0
	if todayPrice > 0 {
		pct = ((sellPrice - todayPrice) / todayPrice) * 100
	}
	sd.Name = "黄金(人民币)"
	sd.Price = sellPrice
	sd.Change = sellPrice - todayPrice
	sd.ChangePct = pct
	return sd
}

func fetchSinaData(codes []string) map[string]StockData {
	result := make(map[string]StockData)
	if len(codes) == 0 {
		return result
	}
	url := fmt.Sprintf("https://hq.sinajs.cn/list=%s", strings.Join(codes, ","))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return result
	}
	req.Header.Set("Referer", "https://finance.sina.com.cn")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result
	}
	re := regexp.MustCompile(`var hq_str_(\w+)="(.+)"`)
	for _, line := range strings.Split(decodeSinaBody(body), "\n") {
		matches := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) < 3 {
			continue
		}
		code := matches[1]
		fields := strings.Split(matches[2], ",")
		if strings.HasPrefix(code, "fx_s") && len(fields) >= 8 {
			buy, _ := strconv.ParseFloat(fields[1], 64)
			sell, _ := strconv.ParseFloat(fields[2], 64)
			price := (buy + sell) / 2
			prev, _ := strconv.ParseFloat(fields[3], 64)
			pct := 0.0
			if prev > 0 {
				pct = ((price - prev) / prev) * 100
			}
			result[code] = StockData{Name: fields[0], Price: price, Change: price - prev, ChangePct: pct}
		} else if strings.HasPrefix(code, "hf_") && len(fields) >= 14 {
			// hf_ 返回格式: [0]现价 [1]空 [2]买价 [3]卖价 [4]最高 [5]最低
			//   [6]时间 [7]开盘 [8]昨收 [9]成交量 [10]持仓 ... [13]名称
			// 涨跌额和涨跌幅需用现价与昨收自行计算
			price, _ := strconv.ParseFloat(fields[0], 64)
			prev, _ := strconv.ParseFloat(fields[8], 64)
			change := price - prev
			pct := 0.0
			if prev > 0 {
				pct = (change / prev) * 100
			}
			result[code] = StockData{Price: price, Change: change, ChangePct: pct}
		} else if len(fields) >= 32 {
			price, _ := strconv.ParseFloat(fields[3], 64)
			prev, _ := strconv.ParseFloat(fields[2], 64)
			change := price - prev
			pct := 0.0
			if prev > 0 {
				pct = (change / prev) * 100
			}
			result[code] = StockData{Name: fields[0], Price: price, Change: change, ChangePct: pct}
		}
	}
	return result
}

func refreshData() {
	if !refreshMu.TryLock() {
		return
	}
	defer refreshMu.Unlock()

	var codes []string
	var goldItems []int // 记录 gold_rmb 在 stocks 中的索引
	nameMap := make(map[string]string)
	for i, s := range config.Stocks {
		if s.Code == "gold_rmb" {
			goldItems = append(goldItems, i)
			continue
		}
		codes = append(codes, s.Code)
		nameMap[s.Code] = s.Name
	}
	data := fetchSinaData(codes)
	goldData := fetchGoldPrice()
	// 判断是否有数据返回（至少一个品种有价格）
	sinaOK := len(data) > 0
	goldOK := goldData.Price > 0
	newStocks := make([]StockData, len(config.Stocks))
	// 先按顺序填充 Sina 数据
	sinaIdx := 0
	for i, s := range config.Stocks {
		if s.Code == "gold_rmb" {
			newStocks[i] = goldData
			continue
		}
		code := s.Code
		if sinaIdx < len(codes) {
			code = codes[sinaIdx]
			sinaIdx++
		}
		if sd, ok := data[code]; ok {
			if sd.Name == "" {
				sd.Name = nameMap[code]
			}
			newStocks[i] = sd
		} else {
			newStocks[i] = StockData{Name: nameMap[code]}
		}
	}
	stockMu.Lock()
	stockData = newStocks
	lastFetchOK = sinaOK || goldOK
	stockMu.Unlock()
	select {
	case <-appDone:
		return
	default:
	}
	if hwnd != 0 {
		pPostMessageW.Call(uintptr(hwnd), WM_REFRESH_DONE, 0, 0)
	}
}

// reloadConfigIfChanged 检测 config.json 是否被外部修改，是则重新加载
func reloadConfigIfChanged() bool {
	fi, err := os.Stat(configPath)
	if err != nil {
		return false
	}
	if !fi.ModTime().After(configMTime) {
		return false
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}
	var newCfg Config
	if err := json.Unmarshal(data, &newCfg); err != nil || len(newCfg.Stocks) == 0 {
		return false
	}
	// 保留窗口位置等运行时状态
	newCfg.PosX = config.PosX
	newCfg.PosY = config.PosY
	newCfg.Width = config.Width
	newCfg.Height = config.Height
	if newCfg.Opacity == 0 {
		newCfg.Opacity = config.Opacity
	}
	if newCfg.ColorMode == "" {
		newCfg.ColorMode = config.ColorMode
	}
	config = newCfg
	configMTime = fi.ModTime()
	// 更新窗口样式和大小
	updateWindowStyle()
	pSetWindowPos.Call(uintptr(hwnd), 0, 0, 0, uintptr(config.Width), uintptr(config.Height), SWP_NOMOVE|SWP_NOZORDER|SWP_SHOWWINDOW)
	return true
}

func dataWorker() {
	refreshData()
	for {
		interval := time.Duration(config.RefreshInterval) * time.Second
		if interval < time.Second {
			interval = 5 * time.Second
		}
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
			reloadConfigIfChanged() // 每轮先检查配置变更
			refreshData()
		case <-appDone:
			timer.Stop()
			return
		}
	}
}

func updateWindowStyle() {
	exStyle := WS_EX_LAYERED | WS_EX_TOOLWINDOW
	if config.TopMost {
		exStyle |= WS_EX_TOPMOST
	}
	pSetWindowLongW.Call(uintptr(hwnd), uintptr(^uintptr(19)), uintptr(exStyle))
	pSetLayeredWindow.Call(uintptr(hwnd), 0, uintptr(config.Opacity), LWA_ALPHA)
	// 用 SetWindowPos 实际改变 Z 序（仅改 style 不会生效）
	hwndInsertAfter := ^uintptr(1) // HWND_NOTOPMOST
	if config.TopMost {
		hwndInsertAfter = ^uintptr(0) // HWND_TOPMOST
	}
	pSetWindowPos.Call(uintptr(hwnd), hwndInsertAfter, 0, 0, 0, 0, 0x0003) // SWP_NOMOVE|SWP_NOSIZE
	pInvalidateRect.Call(uintptr(hwnd), 0, 1)
}

func showMenu() {
	menu, _, _ := pCreatePopupMenu.Call()
	topText := "\u7f6e\u9876: \u5173"
	if config.TopMost {
		topText = "\u7f6e\u9876: \u5f00"
	}
	pAppendMenuW.Call(menu, MF_STRING, MENU_TOPMOST, uintptr(unsafe.Pointer(u16(topText))))
	pAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)
	pAppendMenuW.Call(menu, MF_STRING, MENU_OPACITY_UP, uintptr(unsafe.Pointer(u16("\u900f\u660e\u5ea6 +"))))
	pAppendMenuW.Call(menu, MF_STRING, MENU_OPACITY_DOWN, uintptr(unsafe.Pointer(u16("\u900f\u660e\u5ea6 -"))))
	pAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)
	pAppendMenuW.Call(menu, MF_STRING, MENU_SCALE_UP, uintptr(unsafe.Pointer(u16("\u653e\u5927"))))
	pAppendMenuW.Call(menu, MF_STRING, MENU_SCALE_DOWN, uintptr(unsafe.Pointer(u16("\u7f29\u5c0f"))))
	pAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)
	colorText := "\u6da8\u8dcc\u989c\u8272: \u6da8\u7ea2\u8dcc\u7eff"
	if config.ColorMode == "green_up" {
		colorText = "\u6da8\u8dcc\u989c\u8272: \u6da8\u7eff\u8dcc\u7ea2"
	}
	pAppendMenuW.Call(menu, MF_STRING, MENU_COLOR_MODE, uintptr(unsafe.Pointer(u16(colorText))))
	pAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)
	pAppendMenuW.Call(menu, MF_STRING, MENU_REFRESH, uintptr(unsafe.Pointer(u16("\u5237\u65b0"))))
	pAppendMenuW.Call(menu, MF_STRING, MENU_SETTINGS, uintptr(unsafe.Pointer(u16("\u7f16\u8f91\u80a1\u7968..."))))
	pAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)
	pAppendMenuW.Call(menu, MF_STRING, MENU_ABOUT, uintptr(unsafe.Pointer(u16("\u5173\u4e8e"))))
	pAppendMenuW.Call(menu, MF_STRING, MENU_EXIT, uintptr(unsafe.Pointer(u16("\u9000\u51fa"))))
	var pt POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(uintptr(hwnd))
	pTrackPopupMenu.Call(menu, TPM_RIGHTBUTTON|TPM_BOTTOMALIGN, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hwnd), 0)
	pDestroyMenu.Call(menu)
}

func openSettings() {
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	shell32.NewProc("ShellExecuteW").Call(0,
		uintptr(unsafe.Pointer(u16("open"))),
		uintptr(unsafe.Pointer(u16("notepad.exe"))),
		uintptr(unsafe.Pointer(u16(configPath))), 0, SW_SHOW)
}

// openStockURL 根据股票代码打开对应网页
func openStockURL(code string) {
	var url string
	switch {
	case strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz"):
		url = fmt.Sprintf("https://finance.sina.com.cn/realstock/company/%s/nc.shtml", code)
	case strings.HasPrefix(code, "hf_"):
		url = fmt.Sprintf("https://finance.sina.com.cn/futures/quotes/%s.shtml", code)
	case code == "gold_rmb":
		url = "https://finance.sina.com.cn/gold/"
	default:
		url = "https://finance.sina.com.cn"
	}
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	shell32.NewProc("ShellExecuteW").Call(0,
		uintptr(unsafe.Pointer(u16("open"))),
		uintptr(unsafe.Pointer(u16(url))),
		0, 0, SW_SHOW)
}

func wndProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_NCCALCSIZE:
		// 让非客户区尺寸为零，消除白色边框（WS_THICKFRAME 拖拽缩放仍生效）
		return 0
	case WM_NCHITTEST:
		// WM_NCCALCSIZE 清零后系统 resize 失效，手动检测边缘/角落
		var pt POINT
		pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		pScreenToClient.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pt)))
		var rc RECT
		pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
		edge := int32(6) // 边缘检测区域像素
		onLeft := pt.X >= 0 && pt.X < edge
		onRight := pt.X <= rc.Right && pt.X > rc.Right-edge
		onTop := pt.Y >= 0 && pt.Y < edge
		onBottom := pt.Y <= rc.Bottom && pt.Y > rc.Bottom-edge
		if onTop && onLeft {
			return HTTOPLEFT
		}
		if onTop && onRight {
			return HTTOPRIGHT
		}
		if onBottom && onLeft {
			return HTBOTTOMLEFT
		}
		if onBottom && onRight {
			return HTBOTTOMRIGHT
		}
		if onLeft {
			return HTLEFT
		}
		if onRight {
			return HTRIGHT
		}
		if onTop {
			return HTTOP
		}
		if onBottom {
			return HTBOTTOM
		}
		return 1 // HTCLIENT
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		pSetBkMode.Call(hdc, TRANSPARENT)
		var rc RECT
		pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
		brush, _, _ := pCreateSolidBrush.Call(uintptr(rgb(22, 22, 38)))
		pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), brush)
		pDeleteObject.Call(brush)
		font, _, _ := pCreateFontW.Call(18, 0, 0, 0, FW_NORMAL, 0, 0, 0,
			1, 0, 0, 0, 0, uintptr(unsafe.Pointer(u16("SimSun"))))
		oldFont, _, _ := pSelectObject.Call(hdc, font)
		stockMu.RLock()
		stocks := make([]StockData, len(stockData))
		copy(stocks, stockData)
		fetchOK := lastFetchOK
		stockMu.RUnlock()
		y := int32(10)
		// 网络错误提示
		if !fetchOK && len(stocks) > 0 {
			pSetTextColor.Call(hdc, 0x006666FF)
			drawStr(hdc, 10, y, "网络异常")
			y += 22
		}
		for _, s := range stocks {
			color := uint32(0x00888888)
			sign := " "
			if s.ChangePct > 0 {
				sign = "+"
				if config.ColorMode == "green_up" {
					color = 0x0000CC66 // 涨绿
				} else {
					color = 0x006666FF // 涨红
				}
			} else if s.ChangePct < 0 {
				if config.ColorMode == "green_up" {
					color = 0x006666FF // 跌红
				} else {
					color = 0x0000CC66 // 跌绿
				}
			}
			pSetTextColor.Call(hdc, uintptr(color))
			drawStr(hdc, 10, y, s.Name)
			drawStr(hdc, 150, y, fmt.Sprintf("%.3f", s.Price))
			drawStr(hdc, 230, y, fmt.Sprintf("%s%.2f%%", sign, s.ChangePct))
			y += 22
		}
		pSetTextColor.Call(hdc, 0x00555555)
		drawStr(hdc, 10, y+6, time.Now().Format("15:04:05"))
		pSelectObject.Call(hdc, oldFont)
		pDeleteObject.Call(font)
		pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_REFRESH_DONE:
		pInvalidateRect.Call(uintptr(hwnd), 0, 1)
		return 0
	case WM_LBUTTONDBLCLK:
		// 双击行情行打开对应网页
		yPos := int32(lParam >> 16) // 高16位是 y 坐标
		stockMu.RLock()
		n := len(stockData)
		codes := make([]string, len(config.Stocks))
		for i, s := range config.Stocks {
			codes[i] = s.Code
		}
		stockMu.RUnlock()
		// 起始 y=10，每行 22px，如果网络异常则第一行是提示，偏移+1
		startY := int32(10)
		if !lastFetchOK && n > 0 {
			startY = 32
		}
		if n > 0 && yPos >= startY {
			row := int((yPos - startY) / 22)
			if row >= 0 && row < len(codes) {
				go openStockURL(codes[row])
			}
		}
		return 0
	case WM_LBUTTONDOWN:
		pReleaseCapture.Call()
		pSendMessageW.Call(uintptr(hwnd), WM_NCLBUTTONDOWN, HTCAPTION, 0)
		return 0
	case WM_RBUTTONDOWN:
		showMenu()
		return 0
	case WM_COMMAND:
		switch uint32(wParam) & 0xFFFF {
		case MENU_EXIT:
			pDestroyWindow.Call(uintptr(hwnd))
		case MENU_TOPMOST:
			config.TopMost = !config.TopMost
			updateWindowStyle()
			saveConfig()
		case MENU_OPACITY_UP:
			config.Opacity = max(config.Opacity-20, 50)
			updateWindowStyle()
			saveConfig()
		case MENU_OPACITY_DOWN:
			config.Opacity = min(config.Opacity+20, 255)
			updateWindowStyle()
			saveConfig()
		case MENU_REFRESH:
			go refreshData()
		case MENU_SETTINGS:
			go openSettings()
		case MENU_SCALE_UP:
			config.Width = int(float64(config.Width) * 1.2)
			config.Height = int(float64(config.Height) * 1.2)
			pSetWindowPos.Call(uintptr(hwnd), 0, 0, 0, uintptr(config.Width), uintptr(config.Height), SWP_NOMOVE|SWP_NOZORDER|SWP_SHOWWINDOW)
			saveConfig()
		case MENU_SCALE_DOWN:
			config.Width = int(float64(config.Width) / 1.2)
			config.Height = int(float64(config.Height) / 1.2)
			if config.Width < 200 {
				config.Width = 200
			}
			if config.Height < 60 {
				config.Height = 60
			}
			pSetWindowPos.Call(uintptr(hwnd), 0, 0, 0, uintptr(config.Width), uintptr(config.Height), SWP_NOMOVE|SWP_NOZORDER|SWP_SHOWWINDOW)
			saveConfig()
		case MENU_COLOR_MODE:
			if config.ColorMode == "red_up" {
				config.ColorMode = "green_up"
			} else {
				config.ColorMode = "red_up"
			}
			pInvalidateRect.Call(uintptr(hwnd), 0, 1)
			saveConfig()
		case MENU_ABOUT:
			pMessageBoxW.Call(uintptr(hwnd),
				uintptr(unsafe.Pointer(u16("Stock Board v1.0\n作者: lunan\nhttps://github.com/lunan/stock-board"))),
				uintptr(unsafe.Pointer(u16("关于"))), 0)
		}
		return 0
	case WM_EXITSIZEMOVE:
		// 拖拽/缩放结束，立即保存窗口位置和大小
		var rc RECT
		pGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
		config.PosX = int(rc.Left)
		config.PosY = int(rc.Top)
		config.Width = int(rc.Right - rc.Left)
		config.Height = int(rc.Bottom - rc.Top)
		saveConfig()
		return 0
	case WM_DESTROY:
		stopOnce.Do(func() { close(appDone) })
		var rc RECT
		pGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
		// 重新读取 config，避免覆盖用户在外部的修改
		data, err := os.ReadFile(configPath)
		if err == nil {
			var fileCfg Config
			if json.Unmarshal(data, &fileCfg) == nil && len(fileCfg.Stocks) > 0 {
				fileCfg.PosX = int(rc.Left)
				fileCfg.PosY = int(rc.Top)
				fileCfg.Width = int(rc.Right - rc.Left)
				fileCfg.Height = int(rc.Bottom - rc.Top)
				config = fileCfg
			}
		}
		saveConfig()
		pPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func main() {
	runtime.LockOSThread()
	loadConfig()
	className := u16("StockBoardV4")
	windowTitle := u16("Stock Board")
	hInst, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         0x0008, // CS_DBLCLKS，启用双击消息
		LpfnWndProc:   windows.NewCallback(wndProc),
		HInstance:     windows.Handle(hInst),
		LpszClassName: className,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	exStyle := WS_EX_LAYERED | WS_EX_TOOLWINDOW
	if config.TopMost {
		exStyle |= WS_EX_TOPMOST
	}
	ret, _, _ := pCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		WS_POPUP|WS_VISIBLE|WS_THICKFRAME,
		uintptr(config.PosX), uintptr(config.PosY), uintptr(config.Width), uintptr(config.Height),
		0, 0, hInst, 0,
	)
	hwnd = windows.Handle(ret)
	pSetLayeredWindow.Call(uintptr(hwnd), 0, uintptr(config.Opacity), LWA_ALPHA)
	pShowWindow.Call(uintptr(hwnd), SW_SHOW)
	pUpdateWindow.Call(uintptr(hwnd))
	go dataWorker()
	var msg MSG
	for {
		ret, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 {
			break
		}
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
