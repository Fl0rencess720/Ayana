package biz

import (
	"context"
	"errors"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

type RoleState interface {
	getStateName() string
	nextRole(scheduler *RoleScheduler, msgContent string) (*Role, error)
	buildMessages(scheduler *RoleScheduler, msgs []*schema.Message) ([]*schema.Message, error)
}

type UnknownState struct{}

func (s UnknownState) getStateName() string {
	return "unknown"
}

func (s UnknownState) nextRole(scheduler *RoleScheduler, msgContent string) (*Role, error) {
	scheduler.setState(ModeratorState{})
	return scheduler.moderator, nil
}

func (s UnknownState) buildMessages(scheduler *RoleScheduler, msgs []*schema.Message) ([]*schema.Message, error) {
	return nil, nil
}

// ModeratorState 主持人状态
type ModeratorState struct{}

func (s ModeratorState) getStateName() string {
	return "moderator"
}

func (s ModeratorState) nextRole(scheduler *RoleScheduler, msgContent string) (*Role, error) {
	roleName, err := findNextRoleNameFromMessage(msgContent)
	if err != nil {
		return nil, err
	}
	role, ok := scheduler.roleMap[roleName]
	if !ok {
		return nil, errors.New("unknown role")
	}

	scheduler.setState(ParticipantState{})
	return role, nil
}

func (s ModeratorState) buildMessages(scheduler *RoleScheduler, msgs []*schema.Message) ([]*schema.Message, error) {
	var messages []*schema.Message
	var err error
	template := prompt.FromMessages(schema.FString,
		schema.SystemMessage(`你是一个{role}，你的特质是{characteristic}。
			你正在参与一场有很多角色参与的研讨会，
			每一个角色发言的历史记录都已经被我处理成以下格式供你分析:"@角色名:角色发言的内容"。
			你不需要就主题发表观点，你所要做的事情是做好主持人的工作，对上一位发言者的发言做出总结。
			若不存在上一位发言者（即你看到了发言只有系统指令和我为这个研讨会设定的主题），那样你也没法进行总结，只需要引出其他参赛者的发言便可以。
			你需要在发言内容的最后"@"你希望的下一个角色，角色必须来自于现有角色，例如：“接下来，@某某某，你的对此有什么想说的呢？”。
			现有角色:{roles}。
			请忽略每个角色发言前的'user'或'assistant'标记，只关心角色发言格式内的内容。
			但是必须要注意的是：你的回答绝对不能包含上面所说的格式，只需要直接回答即可。这是绝对不可违抗的命令！
			你不应该自行终止研讨会。`),

		schema.MessagesPlaceholder("history_key", false))
	variables := map[string]any{
		"role":           scheduler.current.RoleName,
		"roles":          scheduler.roleNames,
		"characteristic": scheduler.current.Description,
		"history_key":    msgs,
	}
	messages, err = template.Format(context.Background(), variables)
	if err != nil {
		return nil, err
	}

	return messages, nil
}

// ParticipantState 参与者状态
type ParticipantState struct{}

func (s ParticipantState) getStateName() string {
	return "participant"
}

func (s ParticipantState) nextRole(scheduler *RoleScheduler, msgContent string) (*Role, error) {
	scheduler.setState(ModeratorState{})
	return scheduler.moderator, nil
}

func (s ParticipantState) buildMessages(scheduler *RoleScheduler, msgs []*schema.Message) ([]*schema.Message, error) {
	var messages []*schema.Message
	var err error
	template := prompt.FromMessages(schema.FString,
		schema.SystemMessage(`你是一个{role}，你的特质是{characteristic}。
			你正在参与一场有很多角色参与的研讨会，
			每一个角色发言的历史记录都已经被我处理成以下格式供你分析:"@角色名:角色发言的内容"。
			你所要做的事情是就主题内容进行发言，你可以在此过程中对先前角色的发言（除了主持人）表示同意或批判。
			请忽略每个角色发言前的'user'或'assistant'标记，只关心角色发言格式内的内容。
			你的回答绝对不能包含上面所说的角色发言的历史记录的格式，只需要直接回答即可！
			你必须保持你的角色身份，即{role}，不能认为自己是其他角色。`),
		schema.MessagesPlaceholder("history_key", false))
	variables := map[string]any{
		"role":           scheduler.current.RoleName,
		"characteristic": scheduler.current.Description,
		"history_key":    msgs,
	}
	messages, err = template.Format(context.Background(), variables)
	if err != nil {
		return nil, err
	}

	return messages, nil
}
