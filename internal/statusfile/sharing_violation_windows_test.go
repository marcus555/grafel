//go:build windows

package statusfile

import (
	"fmt"
	"testing"

	"golang.org/x/sys/windows"
)

// TestIsRetryableReplaceError_Windows asserts the Windows classifier matches
// BOTH transient "replace an open file" NTFS errors — ERROR_SHARING_VIOLATION
// (the read-side race the Read path always retried on) and ERROR_ACCESS_DENIED
// (the write-side rename-over-open-file error that TestWrite_ConcurrentSameRepo
// _NoTornRead hit on windows-latest) — while a genuinely unrelated error is
// NOT classified as retryable. This test does not run on the darwin dev host;
// it exists to compile under `GOOS=windows go vet` and to run in Windows CI.
func TestIsRetryableReplaceError_Windows(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"sharing_violation", windows.ERROR_SHARING_VIOLATION, true},
		{"access_denied", windows.ERROR_ACCESS_DENIED, true},
		{"wrapped_access_denied", fmt.Errorf("statusfile: rename: %w", windows.ERROR_ACCESS_DENIED), true},
		{"unrelated", windows.ERROR_FILE_NOT_FOUND, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryableReplaceError(tc.err); got != tc.want {
				t.Errorf("isRetryableReplaceError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
