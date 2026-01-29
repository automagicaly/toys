package entities

import (
	"time"
)

const (
	Annoucement MessageType = 0
	Report      MessageType = 1 << (iota - 1)
	Confirmation
	TeacherNote
	ParentNote
	Urgent
	Event
	Reserved_0
	Reserved_1
)

type MessageContent string
type MessageID uint64
type MessageType uint8
type MessageMetadata struct {
	Student          Optional[Student]
	Teacher          Optional[Staff]
	Class            Optional[Class]
	Expiration       Optional[time.Time]
	RelevantAt       Optional[time.Time]
	Choices          Optional[[]string]
	IsMultipleChoice Optional[bool]
}

type Recipient interface {
	GetIDs() []ID
	Matches(id ID) bool
}

type Message struct {
	Entity
	Type      MessageType
	Timestamp time.Time
	From      User
	To        Recipient
	Content   MessageContent
	Metadata  Optional[MessageMetadata]
}
