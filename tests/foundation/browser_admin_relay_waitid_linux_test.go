//go:build browser && linux

// Purpose: install the Linux waitid(WNOWAIT) reaper used by relay.launch to hold an exited leader (its PGID) unreaped until
// the process group has been signalled (#147 TH-1).
// Depends on: golang.org/x/sys/unix (already in go.mod, indirect; no new module), os/exec.
// Used by: browserAdminRelay.launch and TestBrowserAdminRelaySignalsGroupBeforeReapingLeader.

package foundation_test

import "golang.org/x/sys/unix"

func init() {
	// waitLeaderNoReap blocks until pid exits, but WNOWAIT leaves it an unreaped child: the kernel reserves the pid (and
	// therefore the process group id we kill) until os/exec's Wait reaps it after the group signal.
	waitLeaderNoReap = func(pid int) bool {
		var info unix.Siginfo
		for {
			err := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
			switch err {
			case nil:
				return true
			case unix.EINTR:
				continue
			default:
				return false
			}
		}
	}
	leaderStillWaitable = func(pid int) bool {
		var info unix.Siginfo
		for {
			err := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
			switch err {
			case nil:
				return true
			case unix.EINTR:
				continue
			default:
				return false
			}
		}
	}
}
