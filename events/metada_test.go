package events

import (
	"encoding/json"
	"testing"
)

func TestEventManager(t *testing.T) {
	ev := EventManager{FileId: "0f8773d9-a56e-472d-8252-dc29b8d83fa7"}
	ev.NewDocument("00000000-0000-0000-0000-000000000000")
	ev.FileTypeDetected("Pdf")
	ev.RebuildStarted()
	ev.RebuildCompleted("replace")
	b, err := ev.MarshalJson()
	if err != nil {

		t.Errorf("error marshalling %s", err)
	}
	TestEvent := Metadata{}
	SampleEvent := Metadata{}

	json.Unmarshal(b, &TestEvent)
	json.Unmarshal([]byte(MetaDataJsonSample), &SampleEvent)

	if len(TestEvent.Events) != len(SampleEvent.Events) {
		t.Errorf("not the expected result")
	}

}
