package standaloneWrappers

import (
	"context"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	agents "github.com/movie-guru/pkg/agents"
	db "github.com/movie-guru/pkg/db"
	types "github.com/movie-guru/pkg/types"
	utils "github.com/movie-guru/pkg/utils"
)

type ProfileAgent struct {
	MovieAgentDB *db.MovieAgentDB
	Flow         *genkit.Flow[*types.ProfileAgentInput, *types.UserProfileAgentOutput, struct{}]
}

func CreateProfileAgent(ctx context.Context, model ai.Model, db *db.MovieAgentDB) (*ProfileAgent, error) {
	flow, err := agents.GetUserProfileFlow(ctx, model)
	if err != nil {
		return nil, err
	}
	return &ProfileAgent{
		MovieAgentDB: db,
		Flow:         flow,
	}, nil
}

func (p *ProfileAgent) Run(ctx context.Context, history *types.ChatHistory, user string) (*types.UserProfileOutput, error) {
	userProfile, err := p.MovieAgentDB.GetCurrentProfile(ctx, user)
	userProfileOutput := &types.UserProfileOutput{
		UserProfile: userProfile,
		ChangesMade: false,
		ModelOutputMetadata: &types.ModelOutputMetadata{
			SafetyIssue:   false,
			Justification: "",
		},
	}
	if err != nil {
		return nil, err
	}
	agentMessage := ""
	if len(history.History) > 1 {
		agentMessage = history.History[len(history.History)-2].Content[0].Text
	}
	lastUserMessage, err := history.GetLastMessage()
	if err != nil {
		return nil, err
	}

	prefInput := types.ProfileAgentInput{Query: lastUserMessage, AgentMessage: agentMessage}
	resp, err := p.Flow.Run(ctx, &prefInput)
	if err != nil {
		return userProfileOutput, err
	}
	userProfileOutput.ChangesMade = resp.ChangesMade
	userProfileOutput.ModelOutputMetadata.Justification = resp.ModelOutputMetadata.Justification
	userProfileOutput.ModelOutputMetadata.SafetyIssue = resp.ModelOutputMetadata.SafetyIssue

	if len(resp.ProfileChangeRecommendations) > 0 {
		updatedProfile, err := utils.ProcessProfileChanges(userProfile, resp.ProfileChangeRecommendations)
		if err != nil {
			return userProfileOutput, err
		}
		err = p.MovieAgentDB.UpdateProfile(ctx, updatedProfile, user)
		if err != nil {
			return userProfileOutput, err
		}
		userProfileOutput.UserProfile = updatedProfile
	}
	return userProfileOutput, nil
}
