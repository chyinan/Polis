// pattern: Imperative Shell
package kernel

import "polis/internal/runner"

func stopPGTestPeerWorkers(env peerHardeningEnv) error {
	for _, item := range []struct {
		b Binding
		p interface {
			Stop() (runner.StopProof, error)
		}
	}{{env.backend, env.process}, {env.frontend, env.frontendProcess}} {
		if err := env.k.TXBeginStop(env.ctx, item.b); err != nil {
			return err
		}
		proof, err := item.p.Stop()
		if err != nil {
			return err
		}
		if err = env.k.TXConfirmStopped(env.ctx, item.b, proof); err != nil {
			return err
		}
	}
	return nil
}
