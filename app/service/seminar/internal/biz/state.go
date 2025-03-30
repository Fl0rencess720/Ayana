package biz

import (
	"errors"
)

type State interface {
	getState() string
	start(topic *Topic) error
	pause(topic *Topic) error
	resume(topic *Topic) error
	nextState(topic *Topic)
}

type PreparingState struct{}
type RunningState struct{}
type PausedState struct{}

func (p *PreparingState) getState() string {
	return "preparing"
}

func (p *PreparingState) start(topic *Topic) error {
	return nil
}

func (p *PreparingState) pause(topic *Topic) error {
	return errors.New("cannot pause from preparing state")
}

func (p *PreparingState) resume(topic *Topic) error {
	return errors.New("cannot resume from preparing state")
}

func (p *PreparingState) nextState(topic *Topic) {
	topic.State = &RunningState{}
}

func (p *RunningState) getState() string {
	return "running"
}
func (r *RunningState) start(topic *Topic) error {
	return errors.New("already running")
}

func (r *RunningState) pause(topic *Topic) error {
	return nil
}

func (r *RunningState) resume(topic *Topic) error {
	return errors.New("not paused")
}

func (r *RunningState) nextState(topic *Topic) {
	topic.State = &PausedState{}
}

func (p *PausedState) getState() string {
	return "paused"
}

func (p *PausedState) start(topic *Topic) error {
	return errors.New("use resume to restart")
}

func (p *PausedState) pause(topic *Topic) error {
	return errors.New("already paused")
}

func (p *PausedState) resume(topic *Topic) error {
	return nil
}

func (p *PausedState) nextState(topic *Topic) {
	topic.State = &RunningState{}
}
