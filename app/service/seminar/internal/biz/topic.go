package biz

import (
	"time"

	"github.com/Fl0rencess720/Wittgenstein/pkgs/utils"
)

type Speech struct {
	Uid     string
	RoleUid string
	Content string
	Time    time.Time
}

type Topic struct {
	state        State
	UID          string
	Participants []string
	Speeches     []Speech
	Title        string
	TitleImage   string
}

func NewTopic(participants []string) (*Topic, error) {
	uid, err := utils.GetSnowflakeID(0)
	if err != nil {
		return nil, err
	}
	return &Topic{
		state:        &PreparingState{},
		UID:          uid,
		Participants: participants,
		Speeches:     []Speech{},
		Title:        "新主题",
	}, nil
}

func (topic *Topic) GetState() string {
	return topic.state.getState()
}

func (topic *Topic) Start() error {
	return topic.state.start(topic)
}

func (topic *Topic) Pause() error {
	return topic.state.pause(topic)
}

func (topic *Topic) Resume() error {
	return topic.state.resume(topic)
}
