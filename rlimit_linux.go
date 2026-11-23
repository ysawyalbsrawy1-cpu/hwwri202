//go:build linux

package main

import (
	"log"
	"syscall"
)

// raiseNofileLimit increases the open file descriptor limit as high as the
// kernel allows. Each concurrent card-check holds ~10–15 FDs (TLS sockets,
// pipe ends, timers). With maxWorkers=2000 that's up to 30 000 FDs.
// Modern kernels support 1 048 576; containers default to 1 024 — way too low.
func raiseNofileLimit() {
	var r syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &r); err != nil {
		log.Printf("[api] WARNING: Getrlimit: %v", err)
		return
	}

	// Try 1 048 576 first (2^20); fall back to the hard cap if lower.
	want := uint64(1_048_576)
	if r.Max > 0 && r.Max < want {
		want = r.Max
	}
	r.Cur = want

	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &r); err != nil {
		// Non-fatal: log and continue. The binary still works, just
		// capped at whatever the container default is.
		log.Printf("[api] WARNING: could not raise NOFILE to %d: %v "+
			"(run as root, or add LimitNOFILE=1048576 in systemd / "+
			"ulimits in docker-compose)", want, err)
	} else {
		log.Printf("[api] NOFILE limit raised to %d", r.Cur)
	}
}
