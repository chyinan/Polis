// pattern: Functional Core
package main

import "testing"

func TestValidateNoProviderProtocolAcceptsInitializeAndThreadStart(t *testing.T) {
	protocol := []byte(`{"method":"initialize"}
{"method":"initialized"}
{"id":2,"method":"thread/start"}
{"method":"thread/started"}
`)
	if err := validateNoProviderProtocol(protocol); err != nil {
		t.Fatal(err)
	}
}

func TestValidateNoProviderProtocolRejectsTurn(t *testing.T) {
	protocol := []byte(`{"method":"initialize"}
{"method":"turn/start"}
`)
	if err := validateNoProviderProtocol(protocol); err == nil {
		t.Fatal("provider turn protocol was accepted by no-provider preflight")
	}
}
