//go:build windows

package style

import (
	"syscall"
	"unsafe"
)

const enableVirtualTerminalProcessing = 0x0004

var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	procGetMode  = kernel32.NewProc("GetConsoleMode")
	procSetMode  = kernel32.NewProc("SetConsoleMode")
	procSetOutCP = kernel32.NewProc("SetConsoleOutputCP")
	procSetCP    = kernel32.NewProc("SetConsoleCP")
)

func init() {
	// 让老版 conhost 也认 ANSI 转义序列，并切到 UTF-8 代码页，
	// 否则中文和框线字符会变成乱码。
	h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil || h == syscall.InvalidHandle {
		return
	}
	var mode uint32
	if ok, _, _ := procGetMode.Call(uintptr(h), uintptr(unsafe.Pointer(&mode))); ok == 0 {
		return
	}
	if mode&enableVirtualTerminalProcessing == 0 {
		procSetMode.Call(uintptr(h), uintptr(mode|enableVirtualTerminalProcessing))
	}
	procSetOutCP.Call(65001)
	procSetCP.Call(65001)
}
