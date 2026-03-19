// Package sessions detects other active Claude Code sessions and their state.
//
// Detection uses macOS sysctl to enumerate processes and proc_pidinfo to get
// each session's working directory, keeping the total cost under 10ms — safe
// for the statusline's per-tick budget.
//
// The approach:
//  1. sysctl KERN_PROC_ALL → find claude PIDs by version-pattern p_comm (~0.5ms)
//  2. proc_pidinfo PROC_PIDVNODEPATHINFO → get CWD for each PID (~0.07ms total)
//  3. CWD → project slug → find most recent .jsonl in that project dir (~1ms)
//  4. Tail 2KB of each transcript → check if last tool_use has no tool_result (~0.1ms)
package sessions

/*
#include <libproc.h>
#include <sys/proc_info.h>
*/
import "C"

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unsafe"
)

// versionRe matches the semver string that Claude Code stores in p_comm
// (e.g. "2.1.79"). This is how we identify claude processes without
// reading argv, which would require a second sysctl per PID.
var versionRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// sysctl kinfo_proc layout on arm64 macOS (verified via cgo offsetof).
const (
	kinfoSize  = 648
	pidOffset  = 40
	commOffset = 243
)

// WaitingSession holds information about another Claude Code session that is
// blocked waiting for user permission approval.
type WaitingSession struct {
	CWD     string // full working directory path
	Project string // last path component (e.g. "tail-claude-hud")
}

// FindWaitingSession scans other Claude Code sessions and returns the first one
// that appears to be blocked waiting for user permission approval.
//
// ownTranscript is excluded from checking (it's this session's transcript).
// Returns nil if no session is waiting or if detection fails.
func FindWaitingSession(ownTranscript string) *WaitingSession {
	myPID := int32(os.Getpid())
	myPPID := int32(os.Getppid())

	// Step 1: Find claude PIDs via sysctl.
	pids := findClaudePIDs()
	if len(pids) <= 1 {
		return nil
	}

	// Step 2: For each other PID, get CWD → project slug → transcript.
	projectsDir := filepath.Join(os.Getenv("HOME"), ".claude", "projects")

	for _, pid := range pids {
		if pid == myPID || pid == myPPID {
			continue
		}

		cwd := getProcessCWD(pid)
		if cwd == "" {
			continue
		}

		// Convert CWD to the project slug that Claude Code uses for transcript dirs.
		// e.g. "/Users/kyle/Code/my-project" → "-Users-kyle-Code-my-project"
		slug := cwdToSlug(cwd)
		projDir := filepath.Join(projectsDir, slug)

		transcript := mostRecentTranscript(projDir, ownTranscript)
		if transcript == "" {
			continue
		}

		if isWaitingForPermission(transcript) {
			return &WaitingSession{
				CWD:     cwd,
				Project: filepath.Base(cwd),
			}
		}
	}

	return nil
}

// findClaudePIDs scans the kernel process table for processes whose p_comm
// matches a semver pattern (Claude Code sets p_comm to its version).
func findClaudePIDs() []int32 {
	buf := allProcs()
	if buf == nil {
		return nil
	}

	var pids []int32
	for i := 0; i+kinfoSize <= len(buf); i += kinfoSize {
		comm := buf[i+commOffset : i+commOffset+16]
		if idx := bytes.IndexByte(comm, 0); idx >= 0 {
			comm = comm[:idx]
		}
		if versionRe.Match(comm) {
			pid := *(*int32)(unsafe.Pointer(&buf[i+pidOffset]))
			pids = append(pids, pid)
		}
	}
	return pids
}

// allProcs returns the raw sysctl KERN_PROC_ALL buffer.
func allProcs() []byte {
	mib := [4]int32{1, 14, 0, 0} // CTL_KERN, KERN_PROC, KERN_PROC_ALL
	var size uintptr
	_, _, err := syscall.Syscall6(syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])), 4, 0,
		uintptr(unsafe.Pointer(&size)), 0, 0)
	if err != 0 {
		return nil
	}

	buf := make([]byte, size)
	_, _, err = syscall.Syscall6(syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])), 4,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)), 0, 0)
	if err != 0 {
		return nil
	}
	return buf[:size]
}

// getProcessCWD returns the current working directory of a process using
// proc_pidinfo. Returns "" if the process doesn't exist or access is denied.
func getProcessCWD(pid int32) string {
	var vpi C.struct_proc_vnodepathinfo
	ret := C.proc_pidinfo(C.int(pid), C.PROC_PIDVNODEPATHINFO, 0,
		unsafe.Pointer(&vpi), C.int(unsafe.Sizeof(vpi)))
	if ret <= 0 {
		return ""
	}
	return C.GoString(&vpi.pvi_cdir.vip_path[0])
}

// cwdToSlug converts a filesystem path to the slug format Claude Code uses
// for project directories under ~/.claude/projects/.
// e.g. "/Users/kyle/Code/my-project" → "-Users-kyle-Code-my-project"
func cwdToSlug(cwd string) string {
	return strings.ReplaceAll(cwd, "/", "-")
}

// mostRecentTranscript returns the path to the most recently modified .jsonl
// file in projDir, excluding ownTranscript. Returns "" if none found.
func mostRecentTranscript(projDir string, ownTranscript string) string {
	entries, err := os.ReadDir(projDir)
	if err != nil {
		return ""
	}

	var newest string
	var newestTime int64

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		full := filepath.Join(projDir, e.Name())
		if full == ownTranscript {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if t := fi.ModTime().UnixNano(); t > newestTime {
			newestTime = t
			newest = full
		}
	}
	return newest
}

// isWaitingForPermission reads the tail of a transcript and checks whether
// the last tool_use has no corresponding tool_result — indicating the session
// is blocked waiting for user approval.
func isWaitingForPermission(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return false
	}

	// Read the last 4KB — enough to contain the final tool_use/tool_result pair.
	const tailSize = 4096
	offset := fi.Size() - tailSize
	if offset < 0 {
		offset = 0
	}
	if _, err := f.Seek(offset, 0); err != nil {
		return false
	}

	buf := make([]byte, tailSize)
	n, _ := f.Read(buf)
	tail := buf[:n]

	lastToolUse := bytes.LastIndex(tail, []byte(`"tool_use"`))
	lastToolResult := bytes.LastIndex(tail, []byte(`"tool_result"`))

	return lastToolUse > 0 && lastToolUse > lastToolResult
}
