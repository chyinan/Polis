// pattern: Imperative Shell

package runner

import "fmt"

// StopProof cannot be fabricated by a model or constructed outside this package.
type StopProof struct {
	id      string
	pid     int
	stopped bool
}

func (p StopProof) For(id string) bool { return p.stopped && p.id == id && p.pid > 0 }
func (p StopProof) PID() int           { return p.pid }

func processStopDescription(prefix string, pid int) string {
	return fmt.Sprintf("%s:%d:waited", prefix, pid)
}
