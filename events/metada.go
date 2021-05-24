package events

import (
	"encoding/json"
	"time"
)

const (
	Unknown                 = 0x00
	NewDocument             = 0x10
	FileTypeDetected        = 0x20
	UnmanagedFiletypeAction = 0x30
	RebuildStarted          = 0x40
	BlockedFileTypeAction   = 0x50
	RebuildCompleted        = 0x60
	AnalysisCompleted       = 0x70
	NcfsStartedEvent        = 0x80
	NcfsCompletedEvent      = 0x90
)

const (
	Ti = "2021-05-18T11:03:01.3365613Z"
)

type Metadata struct {
	Events []Events `json:"Events"`
}

type Events struct {
	Properties Properties `json:"Properties,omitempty"`
}

type Properties struct {
	Eventid   int    `json:"EventId"`
	Fileid    string `json:"FileId"`
	Mode      int    `json:"Mode,omitempty"`
	Policyid  string `json:"PolicyId,omitempty"`
	Filetype  string `json:"FileType,omitempty"`
	Gwoutcome string `json:"GwOutcome,omitempty"`
	Timestamp string `json:"Timestamp"`
}

type EventManager struct {
	FileId string
	event  []Events
}

func (e *EventManager) Unknown() {
	p := Properties{
		Eventid:   Unknown,
		Fileid:    e.FileId,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	ev := Events{Properties: p}
	e.event = append(e.event, ev)
}

func (e *EventManager) NewDocument(policyId string) {
	p := Properties{
		Eventid:   NewDocument,
		Fileid:    e.FileId,
		Mode:      1,
		Policyid:  policyId,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	ev := Events{Properties: p}
	e.event = append(e.event, ev)
}

func (e *EventManager) FileTypeDetected(Filetype string) {
	p := Properties{
		Eventid:   FileTypeDetected,
		Fileid:    e.FileId,
		Filetype:  Filetype,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	ev := Events{Properties: p}
	e.event = append(e.event, ev)
}

func (e *EventManager) RebuildStarted() {
	p := Properties{
		Eventid:   RebuildStarted,
		Fileid:    e.FileId,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	ev := Events{Properties: p}
	e.event = append(e.event, ev)
}

func (e *EventManager) RebuildCompleted(GwOutcome string) {
	p := Properties{
		Eventid:   RebuildCompleted,
		Fileid:    e.FileId,
		Gwoutcome: GwOutcome,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	ev := Events{Properties: p}
	e.event = append(e.event, ev)
}

func (e *EventManager) MarshalJson() ([]byte, error) {

	meta := Metadata{
		Events: e.event,
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return nil, err

	}

	return b, nil
}
