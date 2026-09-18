package shim

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const _CREATE_NO_WINDOW = 0x08000000 //nolint:revive

func deleteAfterExit(path string) {
	script := fmt.Sprintf("Wait-Process -Id %d -ErrorAction SilentlyContinue; Remove-Item -LiteralPath '%s'",
		os.Getpid(), strings.ReplaceAll(path, "'", "''"))
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script) //nolint:noctx
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: _CREATE_NO_WINDOW}
	_ = cmd.Start()
}
