package messagecontroller

import (
	"errors"
	"time"

	"lorde.tech/toys/agenda/entities"
)

type MessageController interface {
	Upsert(message *entities.Message) error
}

type MessageRepo interface {
	Upsert(message *entities.Message) error
	MessagesToUser(user *entities.User, startTime time.Time, endTime time.Time) ([]entities.Message, error)
	MessagesRelatedToStudent(student *entities.Student, startTime time.Time, endTime time.Time) ([]entities.Message, error)
}

type UserRepo interface {
	RelatedStudents(user *entities.User) ([]entities.Student, error)
}

type controller struct {
	messageRepo MessageRepo
	userRepo    UserRepo
}

func New(messageRepo MessageRepo, userRepo UserRepo) (MessageController, error) {
	if messageRepo == nil {
		return nil, errors.New("Could not create message controller, because the message repository is missing!")
	}
	return &controller{
		messageRepo: messageRepo,
		userRepo:    userRepo,
	}, nil
}

func (c *controller) Upsert(message *entities.Message) error {
	return c.messageRepo.Upsert(message)
}

func (c *controller) ListRelevantMessagesForUser(user entities.User, timeSpan time.Duration) ([]entities.Message, error) {
	if timeSpan < 0 {
		return nil, errors.New("Time span is negative, but not message in the past is supposed to be relevant!")
	}

	startTime := time.Now()
	endTime := startTime.Add(timeSpan)
	result, err := c.messageRepo.MessagesToUser(&user, startTime, endTime)
	if err != nil {
		return nil, err
	}

	if user.Type == entities.ParentUserType {
		relatedStudents, err := c.userRepo.RelatedStudents(&user)
		if err != nil {
			return nil, err
		}
		for i := range len(relatedStudents) {
			msgs, err := c.messageRepo.MessagesRelatedToStudent(&relatedStudents[i], startTime, endTime)
			if err != nil {
				continue
			}
			result = append(result, msgs...)
		}
	}

	return dedupMessages(result), nil
}

func dedupMessages(msgs []entities.Message) []entities.Message {
	dedupedMessagesByID := make(map[entities.ID]entities.Message)
	for i := range len(msgs) {
		dedupedMessagesByID[msgs[i].ID] = msgs[i]
	}

	result := make([]entities.Message, 0, len(dedupedMessagesByID))
	for id := range dedupedMessagesByID {
		result = append(result, dedupedMessagesByID[id])
	}
	return result
}
