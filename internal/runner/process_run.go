// pattern: Imperative Shell
package runner

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"time"
)

type processStarter func(identity string, argv, environment []string) (*Process, error)

func Run(argv, environment []string, limit time.Duration) ([]byte, error) {
	return runWithProcessStarter(Start, argv, environment, limit)
}

func runWithProcessStarter(start processStarter, argv, environment []string, limit time.Duration) (output []byte, returnErr error) {
	var executionToken [16]byte
	if _, err := rand.Read(executionToken[:]); err != nil {
		return nil, errors.New("could not create a bounded process identity")
	}
	process, err := start("bounded-check-"+hex.EncodeToString(executionToken[:]), argv, environment)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, stopErr := process.Stop()
		if stopErr != nil {
			returnErr = errors.Join(returnErr, stopErr)
		}
	}()
	defer process.Out.Close()
	defer process.Err.Close()
	process.In.Close()
	type result struct {
		output []byte
		err    error
	}
	results := make(chan result, 2)
	for _, reader := range []io.Reader{process.Out, process.Err} {
		go func(reader io.Reader) {
			output, readErr := io.ReadAll(io.LimitReader(reader, 32769))
			if len(output) > 32768 {
				readErr = errors.New("output limit exceeded")
				process.Stop()
			}
			results <- result{output: output, err: readErr}
		}(reader)
	}
	timer := time.NewTimer(limit)
	defer timer.Stop()
	for index := 0; index < 2; index++ {
		select {
		case result := <-results:
			output = append(output, result.output...)
			if result.err != nil {
				return output, result.err
			}
		case <-timer.C:
			return output, errors.New("process time limit exceeded")
		}
	}
	select {
	case <-process.done:
		return output, process.WaitError()
	case <-timer.C:
		return output, errors.New("process time limit exceeded")
	}
}
