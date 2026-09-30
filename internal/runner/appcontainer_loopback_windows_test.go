// pattern: Imperative Shell
//go:build windows

package runner

import (
	"fmt"
	"testing"
	"time"
)

func TestNamedLoopbackMutexSerializesConcurrentCallers(t *testing.T) {
	name := fmt.Sprintf(`Local\Polis-Test-Loopback-%d`, time.Now().UnixNano())
	firstEntered := make(chan struct{})
	firstRelease := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- withNamedWindowsMutex(name, 2*time.Second, func() error {
			close(firstEntered)
			<-firstRelease
			return nil
		})
	}()
	select {
	case <-firstEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("first named mutex caller did not enter")
	}

	secondEntered := make(chan struct{})
	secondRelease := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- withNamedWindowsMutex(name, 2*time.Second, func() error {
			close(secondEntered)
			<-secondRelease
			return nil
		})
	}()
	select {
	case <-secondEntered:
		t.Fatal("second caller entered while the first held the named mutex")
	case <-time.After(100 * time.Millisecond):
	}
	close(firstRelease)
	select {
	case <-secondEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("second named mutex caller did not enter after release")
	}
	close(secondRelease)
	if err := <-firstDone; err != nil {
		t.Fatalf("first named mutex caller failed: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second named mutex caller failed: %v", err)
	}
}
