//go:build windows

package shell

import (
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var conptyProc = windows.NewLazySystemDLL("kernel32.dll").NewProc("CreatePseudoConsole")

type windowsTerm struct {
	once    sync.Once
	in      *os.File
	out     *os.File
	console windows.Handle
}

func (t *windowsTerm) Read(p []byte) (int, error) {
	if t.out == nil {
		return 0, io.EOF
	}
	return t.out.Read(p)
}

func (t *windowsTerm) Write(p []byte) (int, error) {
	if t.in == nil {
		return 0, io.EOF
	}
	return t.in.Write(p)
}

func (t *windowsTerm) SetReadDeadline(deadline time.Time) error {
	if t.out == nil {
		return io.EOF
	}
	return t.out.SetReadDeadline(deadline)
}

func (t *windowsTerm) Resize(cols, rows int) error {
	if t.console == 0 {
		return io.EOF
	}
	return windows.ResizePseudoConsole(t.console, windows.Coord{X: int16(cols), Y: int16(rows)})
}

func (t *windowsTerm) Close() error {
	t.once.Do(func() {
		if t.in != nil {
			_ = t.in.Close()
		}
		if t.out != nil {
			_ = t.out.Close()
		}
		if t.console != 0 {
			windows.ClosePseudoConsole(t.console)
			t.console = 0
		}
	})
	return nil
}

func startPTY(cmd *exec.Cmd, cols, rows int) (terminal, error) {
	if err := conptyProc.Find(); err != nil {
		return nil, ErrUnsupported
	}
	var ptyIn, inputWrite, outputRead, ptyOut windows.Handle
	if err := windows.CreatePipe(&ptyIn, &inputWrite, nil, 0); err != nil {
		return nil, err
	}
	if err := windows.CreatePipe(&outputRead, &ptyOut, nil, 0); err != nil {
		windows.CloseHandle(ptyIn)
		windows.CloseHandle(inputWrite)
		return nil, err
	}
	var console windows.Handle
	err := windows.CreatePseudoConsole(windows.Coord{X: int16(cols), Y: int16(rows)}, ptyIn, ptyOut, 0, &console)
	windows.CloseHandle(ptyIn)
	windows.CloseHandle(ptyOut)
	if err != nil {
		windows.CloseHandle(inputWrite)
		windows.CloseHandle(outputRead)
		return nil, ErrUnsupported
	}
	_ = windows.SetHandleInformation(inputWrite, windows.HANDLE_FLAG_INHERIT, 0)
	_ = windows.SetHandleInformation(outputRead, windows.HANDLE_FLAG_INHERIT, 0)

	attr, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		closeConPTY(console, inputWrite, outputRead)
		return nil, err
	}
	defer attr.Delete()
	if err := attr.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, unsafe.Pointer(&console), unsafe.Sizeof(console)); err != nil {
		closeConPTY(console, inputWrite, outputRead)
		return nil, err
	}

	argv := cmd.Args
	if len(argv) == 0 {
		argv = []string{cmd.Path}
	}
	commandLine, err := windows.UTF16FromString(windows.ComposeCommandLine(argv))
	if err != nil {
		closeConPTY(console, inputWrite, outputRead)
		return nil, err
	}
	envBlock, err := windowsEnv(cmd.Env)
	if err != nil {
		closeConPTY(console, inputWrite, outputRead)
		return nil, err
	}
	var dir *uint16
	if cmd.Dir != "" {
		dir, err = windows.UTF16PtrFromString(cmd.Dir)
		if err != nil {
			closeConPTY(console, inputWrite, outputRead)
			return nil, err
		}
	}
	var envPtr *uint16
	if len(envBlock) > 0 {
		envPtr = &envBlock[0]
	}

	si := windows.StartupInfoEx{}
	si.Cb = uint32(unsafe.Sizeof(si))
	si.ProcThreadAttributeList = attr.List()
	var pi windows.ProcessInformation
	err = windows.CreateProcess(
		nil,
		&commandLine[0],
		nil,
		nil,
		false,
		windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_UNICODE_ENVIRONMENT,
		envPtr,
		dir,
		&si.StartupInfo,
		&pi,
	)
	runtime.KeepAlive(commandLine)
	runtime.KeepAlive(envBlock)
	runtime.KeepAlive(dir)
	if err != nil {
		closeConPTY(console, inputWrite, outputRead)
		return nil, err
	}
	windows.CloseHandle(pi.Thread)
	windows.CloseHandle(pi.Process)

	proc, err := os.FindProcess(int(pi.ProcessId))
	if err != nil {
		closeConPTY(console, inputWrite, outputRead)
		_ = killProcess(int(pi.ProcessId))
		return nil, err
	}
	cmd.Process = proc
	in := os.NewFile(uintptr(inputWrite), "conpty-in")
	out := os.NewFile(uintptr(outputRead), "conpty-out")
	if in == nil || out == nil {
		if in != nil {
			_ = in.Close()
		}
		if out != nil {
			_ = out.Close()
		}
		windows.ClosePseudoConsole(console)
		_ = killProcess(int(pi.ProcessId))
		return nil, ErrUnsupported
	}
	return &windowsTerm{in: in, out: out, console: console}, nil
}

func closeConPTY(console, inputWrite, outputRead windows.Handle) {
	if console != 0 {
		windows.ClosePseudoConsole(console)
	}
	if inputWrite != 0 {
		windows.CloseHandle(inputWrite)
	}
	if outputRead != 0 {
		windows.CloseHandle(outputRead)
	}
}

func windowsEnv(env []string) ([]uint16, error) {
	if len(env) == 0 {
		return nil, nil
	}
	buf := make([]uint16, 0, len(env)*16)
	for _, item := range env {
		if strings.ContainsRune(item, 0) || !strings.Contains(item, "=") {
			continue
		}
		part, err := windows.UTF16FromString(item)
		if err != nil {
			return nil, err
		}
		buf = append(buf, part...)
	}
	if len(buf) == 0 {
		return nil, nil
	}
	return append(buf, 0), nil
}

func killProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	return windows.TerminateProcess(handle, 1)
}
