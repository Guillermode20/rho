//go:build windows
// +build windows

package agentutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

// WindowsSelfUpdate handles updating a running executable on Windows.
type WindowsSelfUpdate struct{}

// IsMarkedForCleanup checks if the current executable was a downloaded update
// that needs to replace the original.
func (wsu *WindowsSelfUpdate) IsMarkedForCleanup() bool {
	// Check for a .pendingrename file
	execPath, _ := os.Executable()
	pendingFile := execPath + ".pendingrename"
	if _, err := os.Stat(pendingFile); err == nil {
		return true
	}
	return false
}

// ReplaceRunning replaces the running executable with the provided binary data.
// Uses Windows MoveFileEx with MOVEFILE_DELAY_UNTIL_REBOOT for the running executable,
// and immediately replaces the file for the new one.
func (wsu *WindowsSelfUpdate) ReplaceRunning(newBinary []byte) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot get executable path: %w", err)
	}

	// Write the new binary to a temp file
	tmpFile := execPath + ".new.exe"
	if err := os.WriteFile(tmpFile, newBinary, 0755); err != nil {
		return fmt.Errorf("cannot write update: %w", err)
	}

	// Create a rename script that runs after we exit
	renameScript := execPath + ".rename.bat"
	script := fmt.Sprintf(`@echo off
:wait
ping -n 2 127.0.0.1 >nul
if exist "%s" (
    move /y "%s" "%s" >nul
    del "%%~f0"
    exit /b 0
)
goto wait
`, tmpFile, tmpFile, execPath)

	if err := os.WriteFile(renameScript, []byte(script), 0755); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("cannot create rename script: %w", err)
	}

	// Launch the rename script detached
	cmd := exec.Command("cmd.exe", "/c", "start", "/b", renameScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		os.Remove(tmpFile)
		os.Remove(renameScript)
		return fmt.Errorf("cannot start rename script: %w", err)
	}

	return nil
}

// CleanupWindowsSelfUpdateQuarantine cleans up Windows security quarantine markers.
func CleanupWindowsSelfUpdateQuarantine() error {
	// Windows may mark downloaded executables as quarantined (Zone.Identifier ADS)
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	zoneIDFile := execPath + ":Zone.Identifier"
	if _, err := os.Stat(zoneIDFile); err == nil {
		return os.Remove(zoneIDFile)
	}
	return nil
}

// ScheduleUpdate schedules an update to be applied on next restart.
func ScheduleUpdate(newBinaryPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	// Use MoveFileEx with MOVEFILE_DELAY_UNTIL_REBOOT
	oldPath := execPath + ".old.exe"
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	moveFileEx := kernel32.NewProc("MoveFileExW")

	// Rename current to .old
	oldPtr, _ := syscall.UTF16PtrFromString(oldPath)
	execPtr, _ := syscall.UTF16PtrFromString(execPath)
	moveFileEx.Call(uintptr(unsafe.Pointer(execPtr)), uintptr(unsafe.Pointer(oldPtr)), 1) // MOVEFILE_REPLACE_EXISTING

	// Schedule new binary to replace on reboot
	newPtr, _ := syscall.UTF16PtrFromString(newBinaryPath)
	moveFileEx.Call(uintptr(unsafe.Pointer(newPtr)), uintptr(unsafe.Pointer(execPtr)), 5) // MOVEFILE_DELAY_UNTIL_REBOOT | MOVEFILE_REPLACE_EXISTING

	return nil
}

// WaitForUpdateCompletion waits for a pending update to complete.
func WaitForUpdateCompletion(maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		if !IsUpdatePending() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("update did not complete within %v", maxWait)
}

// IsUpdatePending checks if an update operation is still pending.
func IsUpdatePending() bool {
	// Check for pending rename files
	pattern := filepath.Join(os.TempDir(), "*.pendingrename")
	matches, _ := filepath.Glob(pattern)
	return len(matches) > 0
}
