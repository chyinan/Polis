// pattern: Functional Core
package codex

import (
	"encoding/json"
	"testing"
)

func TestValidTurnEventRejectsEventsFromAnotherTurn(t *testing.T) {
	message := Message{Method: "item/agentMessage/delta", Params: json.RawMessage(`{"threadId":"other-thread","turnId":"other-turn","delta":"unexpected"}`)}
	if validTurnEvent(message, "thread-1", "turn-1") {
		t.Fatal("accepted an event from a different thread/turn")
	}
}
