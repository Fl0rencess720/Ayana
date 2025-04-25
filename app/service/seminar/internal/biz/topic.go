package biz

import (
	"database/sql/driver"
	"encoding/json"
	"sync"
	"time"

	"github.com/Fl0rencess720/Wittgenstein/pkgs/utils"
	"gorm.io/gorm"
)

type StateSignal uint8

const (
	Pause StateSignal = iota
	Resume
	Error
	Normal
)

type TopicCache struct {
	sync.RWMutex
	Topics map[string]*Topic
}

type Topic struct {
	gorm.Model
	UID          string   `gorm:"index;column:uid;type:varchar(255)"`
	Content      string   `gorm:"column:content;type:text"`
	State        State    `gorm:"-;"`
	Moderator    string   `gorm:"column:moderator;type:varchar(255)"`
	Participants []string `gorm:"column:participants;type:json;serializer:json"`
	Speeches     []Speech `gorm:"foreignKey:TopicUID;references:UID"`
	Title        string   `gorm:"column:title;type:varchar(255)"`
	TitleImage   string   `gorm:"column:title_image;type:varchar(255)"`
	Phone        string   `gorm:"column:phone;type:varchar(255)"`

	signalChan chan StateSignal `gorm:"-;"`
}

type Speech struct {
	gorm.Model
	UID      string    `gorm:"index;column:uid;type:varchar(255)"`
	TopicUID string    `gorm:"column:topic_uid;type:varchar(255)"`
	RoleUID  string    `gorm:"column:role_uid;type:varchar(50)"`
	RoleName string    `gorm:"column:role_name;type:varchar(50)"`
	Content  string    `gorm:"column:content;type:text"`
	Time     time.Time `gorm:"column:time"`
}

func (t *Topic) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), &t.Participants)
}

func (t Topic) Value() (driver.Value, error) {
	return json.Marshal(t.Participants)
}

func NewTopic(content, moderator string, participants []string) (*Topic, error) {
	uid, err := utils.GetSnowflakeID(0)
	if err != nil {
		return nil, err
	}
	return &Topic{
		Content:      content,
		State:        &PreparingState{},
		UID:          uid,
		Moderator:    moderator,
		Participants: participants,
		Speeches:     []Speech{},
		Title:        "新主题",
		signalChan:   make(chan StateSignal, 1),
	}, nil
}

func (topic *Topic) GetState() string {
	return topic.State.getState()
}

func (topic *Topic) Start() error {
	return topic.State.start(topic)
}

func (topic *Topic) Pause() error {
	return topic.State.pause(topic)
}

func (topic *Topic) Resume() error {
	return topic.State.resume(topic)
}

func NewTopicCache() *TopicCache {
	return &TopicCache{
		Topics: make(map[string]*Topic),
	}
}

func (tc *TopicCache) GetTopic(topicUID string) (*Topic, error) {
	tc.RLock()
	defer tc.RUnlock()
	if topic, ok := tc.Topics[topicUID]; ok {
		return topic, nil
	}
	return nil, nil
}

func (tc *TopicCache) SetTopic(topic *Topic) {
	tc.Lock()
	defer tc.Unlock()
	tc.Topics[topic.UID] = topic
}

func (tc *TopicCache) DeleteTopic(topicUID string) {
	tc.Lock()
	defer tc.Unlock()
	delete(tc.Topics, topicUID)
}
