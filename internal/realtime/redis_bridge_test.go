package realtime

import "testing"

func TestDistributedEventRoundTrip(t *testing.T) {
	payload, err := encodeDistributedEvent(7, "judge_result", map[string]any{
		"submission_id": 42,
		"status":        "AC",
	})
	if err != nil {
		t.Fatalf("encodeDistributedEvent() error = %v", err)
	}

	event, data, err := decodeDistributedEvent(payload)
	if err != nil {
		t.Fatalf("decodeDistributedEvent() error = %v", err)
	}
	if event.UserID != 7 || event.Name != "judge_result" {
		t.Fatalf("event = %#v", event)
	}
	decoded, ok := data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", data)
	}
	if decoded["status"] != "AC" || decoded["submission_id"] != float64(42) {
		t.Fatalf("data = %#v", decoded)
	}
}

func TestDistributedEventRejectsInvalidEnvelope(t *testing.T) {
	for _, payload := range [][]byte{
		[]byte(`not-json`),
		[]byte(`{"user_id":0,"name":"judge_result","data":{}}`),
		[]byte(`{"user_id":1,"name":"","data":{}}`),
		[]byte(`{"user_id":1,"name":"judge_result","data":`),
	} {
		if _, _, err := decodeDistributedEvent(payload); err == nil {
			t.Fatalf("decodeDistributedEvent(%q) unexpectedly succeeded", payload)
		}
	}
}
