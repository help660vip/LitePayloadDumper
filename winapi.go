package main

import (
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	wmCreate    = 0x0001
	wmDestroy   = 0x0002
	wmSize      = 0x0005
	wmSetFocus  = 0x0007
	wmClose     = 0x0010
	wmGetMinMax = 0x0024
	wmCommand   = 0x0111
	wmNotify    = 0x004E
	wmDropFiles = 0x0233
	wmSetFont   = 0x0030
	wmApp       = 0x8000

	wsOverlappedWindow = 0x00CF0000
	wsVisible          = 0x10000000
	wsChild            = 0x40000000
	wsTabStop          = 0x00010000
	wsVScroll          = 0x00200000
	wsHScroll          = 0x00100000
	wsBorder           = 0x00800000
	wsClipChildren     = 0x02000000

	wsExClientEdge  = 0x00000200
	wsExAcceptFiles = 0x00000010

	bsPushButton    = 0x00000000
	bsDefPushButton = 0x00000001
	bsAutoCheckbox  = 0x00000003

	esAutoHScroll = 0x0080
	esAutoVScroll = 0x0040
	esMultiline   = 0x0004
	esReadOnly    = 0x0800
	esNumber      = 0x2000

	ssLeft   = 0x00000000
	ssCenter = 0x00000001

	cbsDropDownList = 0x0003
	cbsHasStrings   = 0x0200

	lvsReport          = 0x0001
	lvsShowSelAlways   = 0x0008
	lvsExFullRowSelect = 0x00000020
	lvsExCheckBoxes    = 0x00000004
	lvsExDoubleBuffer  = 0x00010000

	swShow       = 5
	cwUseDefault = 0x80000000
	colorBtnFace = 15
	idcArrow     = 32512

	mbOK           = 0x00000000
	mbIconError    = 0x00000010
	mbIconInfo     = 0x00000040
	mbYesNo        = 0x00000004
	mbIconQuestion = 0x00000020
	idYes          = 6

	bnClicked = 0
	enChange  = 0x0300

	emSetSel       = 0x00B1
	emReplaceSel   = 0x00C2
	emSetLimitText = 0x00C5

	cbAddString = 0x0143
	cbGetCurSel = 0x0147
	cbSetCurSel = 0x014E

	lvmFirst            = 0x1000
	lvmGetItemCount     = lvmFirst + 4
	lvmDeleteAllItems   = lvmFirst + 9
	lvmGetItemState     = lvmFirst + 44
	lvmSetItemState     = lvmFirst + 43
	lvmSetExtendedStyle = lvmFirst + 54
	lvmInsertItemW      = lvmFirst + 77
	lvmInsertColumnW    = lvmFirst + 97
	lvmSetItemTextW     = lvmFirst + 116

	lvifText           = 0x0001
	lvifState          = 0x0008
	lvisStateImageMask = 0xF000
	lvcfFmt            = 0x0001
	lvcfWidth          = 0x0002
	lvcfText           = 0x0004
	lvcfSubItem        = 0x0008
	lvcfmtLeft         = 0x0000
	lvcfmtRight        = 0x0001

	pbmSetPos     = 0x0402
	pbmSetRange32 = 0x0406

	ofnExplorer      = 0x00080000
	ofnFileMustExist = 0x00001000
	ofnPathMustExist = 0x00000800
	ofnNoChangeDir   = 0x00000008

	bifReturnOnlyFSDirs = 0x0001
	bifNewDialogStyle   = 0x0040

	defaultCharset = 1
	fwNormal       = 400
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	Private uint32
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSmall  uintptr
}

type minMaxInfo struct {
	Reserved     point
	MaxSize      point
	MaxPosition  point
	MinTrackSize point
	MaxTrackSize point
}

type initCommonControlsEx struct {
	Size uint32
	ICC  uint32
}

type lvColumn struct {
	Mask         uint32
	Fmt          int32
	Cx           int32
	Text         *uint16
	TextMax      int32
	SubItem      int32
	Image        int32
	Order        int32
	MinWidth     int32
	DefaultWidth int32
	IdealWidth   int32
}

type lvItem struct {
	Mask      uint32
	Item      int32
	SubItem   int32
	State     uint32
	StateMask uint32
	Text      *uint16
	TextMax   int32
	Image     int32
	Param     uintptr
	Indent    int32
	GroupID   int32
	Columns   uint32
	ColumnPtr *uint32
	ColumnFmt *int32
	Group     int32
}

type openFileName struct {
	Size             uint32
	Owner            uintptr
	Instance         uintptr
	Filter           *uint16
	CustomFilter     *uint16
	MaxCustomFilter  uint32
	FilterIndex      uint32
	File             *uint16
	MaxFile          uint32
	FileTitle        *uint16
	MaxFileTitle     uint32
	InitialDir       *uint16
	Title            *uint16
	Flags            uint32
	FileOffset       uint16
	FileExtension    uint16
	DefaultExtension *uint16
	CustomData       uintptr
	Hook             uintptr
	TemplateName     *uint16
	Reserved         uintptr
	Reserved2        uint32
	FlagsEx          uint32
}

type browseInfo struct {
	Owner       uintptr
	Root        uintptr
	DisplayName *uint16
	Title       *uint16
	Flags       uint32
	Callback    uintptr
	Param       uintptr
	Image       int32
}

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")

	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procSendMessage         = user32.NewProc("SendMessageW")
	procPostMessage         = user32.NewProc("PostMessageW")
	procSetWindowText       = user32.NewProc("SetWindowTextW")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	procMoveWindow          = user32.NewProc("MoveWindow")
	procEnableWindow        = user32.NewProc("EnableWindow")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procLoadCursor          = user32.NewProc("LoadCursorW")
	procMessageBox          = user32.NewProc("MessageBoxW")
	procSetFocus            = user32.NewProc("SetFocus")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")

	procGetModuleHandle      = kernel32.NewProc("GetModuleHandleW")
	procCreateFont           = gdi32.NewProc("CreateFontW")
	procDeleteObject         = gdi32.NewProc("DeleteObject")
	procInitCommonControlsEx = comctl32.NewProc("InitCommonControlsEx")
	procGetOpenFileName      = comdlg32.NewProc("GetOpenFileNameW")
	procSHBrowseForFolder    = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDList  = shell32.NewProc("SHGetPathFromIDListW")
	procDragAcceptFiles      = shell32.NewProc("DragAcceptFiles")
	procDragQueryFile        = shell32.NewProc("DragQueryFileW")
	procDragFinish           = shell32.NewProc("DragFinish")
	procCoTaskMemFree        = ole32.NewProc("CoTaskMemFree")
)

func wstr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func utf16Buffer(s string) []uint16 {
	result := utf16.Encode([]rune(s))
	return append(result, 0)
}

func loword(value uintptr) uint16      { return uint16(value & 0xffff) }
func hiword(value uintptr) uint16      { return uint16((value >> 16) & 0xffff) }
func signedLoword(value uintptr) int32 { return int32(int16(value & 0xffff)) }
func signedHiword(value uintptr) int32 { return int32(int16((value >> 16) & 0xffff)) }

func sendMessage(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	result, _, _ := procSendMessage.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

func postMessage(hwnd uintptr, message uint32, wParam, lParam uintptr) {
	procPostMessage.Call(hwnd, uintptr(message), wParam, lParam)
}

func setText(hwnd uintptr, text string) {
	procSetWindowText.Call(hwnd, uintptr(unsafe.Pointer(wstr(text))))
}

func getText(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLength.Call(hwnd)
	buffer := make([]uint16, int(length)+1)
	if len(buffer) > 0 {
		procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	}
	return syscall.UTF16ToString(buffer)
}

func move(hwnd uintptr, x, y, width, height int32) {
	procMoveWindow.Call(hwnd, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 1)
}

func enable(hwnd uintptr, enabled bool) {
	value := uintptr(0)
	if enabled {
		value = 1
	}
	procEnableWindow.Call(hwnd, value)
}

func createWindow(exStyle uint32, className, title string, style uint32, x, y, width, height int32, parent uintptr, id int, instance uintptr) uintptr {
	hwnd, _, _ := procCreateWindowEx.Call(
		uintptr(exStyle), uintptr(unsafe.Pointer(wstr(className))), uintptr(unsafe.Pointer(wstr(title))), uintptr(style),
		uintptr(x), uintptr(y), uintptr(width), uintptr(height), parent, uintptr(id), instance, 0,
	)
	return hwnd
}

func createUIFont() uintptr {
	fontHeight := int32(-17)
	font, _, _ := procCreateFont.Call(
		uintptr(fontHeight), 0, 0, 0, fwNormal, 0, 0, 0, defaultCharset, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(wstr("Microsoft YaHei UI"))),
	)
	return font
}

func applyFont(hwnd, font uintptr) {
	sendMessage(hwnd, wmSetFont, font, 1)
}

func showMessage(owner uintptr, title, text string, flags uint32) int {
	result, _, _ := procMessageBox.Call(owner, uintptr(unsafe.Pointer(wstr(text))), uintptr(unsafe.Pointer(wstr(title))), uintptr(flags))
	return int(result)
}

func chooseInputFile(owner uintptr) string {
	fileBuffer := make([]uint16, 32768)
	filter := utf16Buffer("OTA / Payload 文件 (*.zip;*.bin)\x00*.zip;*.bin\x00全部文件 (*.*)\x00*.*\x00")
	title := wstr("选择 OTA ZIP 或 payload.bin")
	of := openFileName{
		Owner:       owner,
		Filter:      &filter[0],
		FilterIndex: 1,
		File:        &fileBuffer[0],
		MaxFile:     uint32(len(fileBuffer)),
		Title:       title,
		Flags:       ofnExplorer | ofnFileMustExist | ofnPathMustExist | ofnNoChangeDir,
	}
	of.Size = uint32(unsafe.Sizeof(of))
	ok, _, _ := procGetOpenFileName.Call(uintptr(unsafe.Pointer(&of)))
	if ok == 0 {
		return ""
	}
	return syscall.UTF16ToString(fileBuffer)
}

func chooseFolder(owner uintptr, title string) string {
	display := make([]uint16, 32768)
	bi := browseInfo{
		Owner:       owner,
		DisplayName: &display[0],
		Title:       wstr(title),
		Flags:       bifReturnOnlyFSDirs | bifNewDialogStyle,
	}
	pidl, _, _ := procSHBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return ""
	}
	defer procCoTaskMemFree.Call(pidl)
	pathBuffer := make([]uint16, 32768)
	ok, _, _ := procSHGetPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&pathBuffer[0])))
	if ok == 0 {
		return ""
	}
	return syscall.UTF16ToString(pathBuffer)
}

func getClientSize(hwnd uintptr) (int32, int32) {
	var area rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&area)))
	return area.Right - area.Left, area.Bottom - area.Top
}

func appendEditText(hwnd uintptr, text string) {
	length, _, _ := procGetWindowTextLength.Call(hwnd)
	sendMessage(hwnd, emSetSel, length, length)
	buffer := wstr(text)
	sendMessage(hwnd, emReplaceSel, 0, uintptr(unsafe.Pointer(buffer)))
}

func addListColumn(hwnd uintptr, index int, title string, width int32, right bool) {
	format := int32(lvcfmtLeft)
	if right {
		format = lvcfmtRight
	}
	column := lvColumn{
		Mask:    lvcfFmt | lvcfWidth | lvcfText | lvcfSubItem,
		Fmt:     format,
		Cx:      width,
		Text:    wstr(title),
		SubItem: int32(index),
	}
	sendMessage(hwnd, lvmInsertColumnW, uintptr(index), uintptr(unsafe.Pointer(&column)))
}

func addListItem(hwnd uintptr, row int, columns []string, checked bool) {
	if len(columns) == 0 {
		return
	}
	item := lvItem{Mask: lvifText, Item: int32(row), Text: wstr(columns[0])}
	sendMessage(hwnd, lvmInsertItemW, 0, uintptr(unsafe.Pointer(&item)))
	for column := 1; column < len(columns); column++ {
		sub := lvItem{SubItem: int32(column), Text: wstr(columns[column])}
		sendMessage(hwnd, lvmSetItemTextW, uintptr(row), uintptr(unsafe.Pointer(&sub)))
	}
	setListChecked(hwnd, row, checked)
}

func setListChecked(hwnd uintptr, row int, checked bool) {
	state := uint32(0x1000)
	if checked {
		state = 0x2000
	}
	item := lvItem{StateMask: lvisStateImageMask, State: state}
	sendMessage(hwnd, lvmSetItemState, uintptr(row), uintptr(unsafe.Pointer(&item)))
}

func listItemChecked(hwnd uintptr, row int) bool {
	state := sendMessage(hwnd, lvmGetItemState, uintptr(row), lvisStateImageMask)
	return state&0x2000 != 0
}

func listItemCount(hwnd uintptr) int {
	return int(sendMessage(hwnd, lvmGetItemCount, 0, 0))
}

func draggedFile(drop uintptr) string {
	defer procDragFinish.Call(drop)
	length, _, _ := procDragQueryFile.Call(drop, 0, 0, 0)
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, int(length)+1)
	procDragQueryFile.Call(drop, 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return syscall.UTF16ToString(buffer)
}
