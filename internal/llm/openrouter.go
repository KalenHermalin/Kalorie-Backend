package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"main/internal/apperrors"
	"main/internal/models"
	"strings"
	"time"

	openrouter "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/optionalnullable"
)

type OpenRouterProvider struct {
	client *openrouter.OpenRouter
	model  string
}

func NewOpenRouterProvider(ctx context.Context, apiKey string, model string) *OpenRouterProvider {
	s := openrouter.New(
		openrouter.WithSecurity(apiKey),
	)
	return &OpenRouterProvider{client: s, model: model}
}

func (gm *OpenRouterProvider) GetTimeoutTime() time.Duration {
	return time.Second * 10
}
func (gm *OpenRouterProvider) AnalyzePicture(ctx context.Context, picture []byte, description, systemPrompt string) (*models.MealPayload, error) {
	//TODO: Seperate concerns, meaning build the genAI parts in sub function and return?
	if len(strings.TrimSpace(description)) == 0 {
		description = ""
	}
	userParts := components.CreateChatMessagesUser(
		components.ChatUserMessage{
			Content: components.CreateChatUserMessageContentArrayOfChatContentItems(
				[]components.ChatContentItems{
					components.CreateChatContentItemsText(
						components.ChatContentText{
							Type: components.ChatContentTextTypeText,
							Text: description,
						}),
					components.CreateChatContentItemsImageURL(components.ChatContentImage{
						Type: components.ChatContentImageTypeImageURL,
						ImageURL: components.ChatContentImageImageURL{
							URL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(picture),
						},
					}),
				},
			),
		},
	)

	var systemParts components.ChatMessages
	if len(strings.TrimSpace(systemPrompt)) > 0 {
		systemParts = components.CreateChatMessagesSystem(
			components.ChatSystemMessage{
				Content: components.CreateChatSystemMessageContentStr(systemPrompt),
				Role:    components.ChatSystemMessageRoleSystem,
			},
		)

	}

	model := "openrouter/free"
	isStrict := true
	isStreaming := false
	requireParameters := true
	preferences := components.ProviderPreferences{
		RequireParameters: optionalnullable.From[bool](&requireParameters),
	}
	res, err := gm.client.Chat.Send(ctx, components.ChatRequest{
		Model: &model,
		Messages: []components.ChatMessages{
			systemParts,
			userParts,
		},
		Stream: &isStreaming,
		ResponseFormat: &components.ResponseFormat{
			Type: "json_schema",
			ChatFormatJSONSchemaConfig: &components.ChatFormatJSONSchemaConfig{
				Type: components.ChatFormatJSONSchemaConfigTypeJSONSchema,
				JSONSchema: components.ChatJSONSchemaConfig{
					Name:   "Macros",
					Strict: optionalnullable.From[bool](&isStrict),
					Schema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"success": map[string]any{"type": "boolean"},
							"payload": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"food_name": map[string]any{"type": "string"},
									"cal":       map[string]any{"type": "integer"},
									"fat":       map[string]any{"type": "integer"},
									"protein":   map[string]any{"type": "integer"},
									"carbs":     map[string]any{"type": "integer"},
								},
							},
						},
					},
				},
			},
		},
		Provider: optionalnullable.OptionalNullable[components.ProviderPreferences]{
			true: &preferences,
		},
	}, nil)
	if err != nil {
		return nil, apperrors.ErrLLMProvider
	}
	var response models.MealResponse
	if len(res.ChatResult.Choices) == 0 {
		return nil, apperrors.ErrLLMProvider
	}
	content, ok := res.ChatResult.Choices[0].Message.Content.Get()
	if !ok || content == nil {
		return nil, apperrors.ErrLLMProvider
	}

	if content.Type != components.ChatAssistantMessageContentTypeStr || content.Str == nil {
		return nil, apperrors.ErrLLMProvider
	}
	responseText := *content.Str

	err = json.Unmarshal([]byte(responseText), &response)
	if err != nil {
		return nil, err
	}

	if !response.Success {
		return nil, apperrors.ErrNoFoodFound
	}
	return &response.Payload, nil

}

func (gm *OpenRouterProvider) AnalyzeLabel(ctx context.Context, picture []byte, systemPrompt string) (*models.LabelPayload, error) {
	//TODO: Seperate concerns, meaning build the genAI parts in sub function and return?

	userParts := components.CreateChatMessagesUser(
		components.ChatUserMessage{
			Content: components.CreateChatUserMessageContentArrayOfChatContentItems(
				[]components.ChatContentItems{
					components.CreateChatContentItemsImageURL(components.ChatContentImage{
						Type: components.ChatContentImageTypeImageURL,
						ImageURL: components.ChatContentImageImageURL{
							URL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(picture),
						},
					}),
				},
			),
		},
	)

	var systemParts components.ChatMessages
	if len(strings.TrimSpace(systemPrompt)) > 0 {
		systemParts = components.CreateChatMessagesSystem(
			components.ChatSystemMessage{
				Content: components.CreateChatSystemMessageContentStr(systemPrompt),
				Role:    components.ChatSystemMessageRoleSystem,
			},
		)

	}

	model := "openrouter/free"
	isStrict := true
	isStreaming := false
	requireParameters := true
	preferences := components.ProviderPreferences{
		RequireParameters: optionalnullable.From[bool](&requireParameters),
	}
	res, err := gm.client.Chat.Send(ctx, components.ChatRequest{
		Model: &model,
		Messages: []components.ChatMessages{
			systemParts,
			userParts,
		},
		Stream: &isStreaming,
		ResponseFormat: &components.ResponseFormat{
			Type: "json_schema",
			ChatFormatJSONSchemaConfig: &components.ChatFormatJSONSchemaConfig{
				Type: components.ChatFormatJSONSchemaConfigTypeJSONSchema,
				JSONSchema: components.ChatJSONSchemaConfig{
					Name:   "Macros",
					Strict: optionalnullable.From[bool](&isStrict),
					Schema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"success": map[string]any{"type": "boolean"},
							"payload": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"units":         map[string]any{"type": "string"},
									"base_quantity": map[string]any{"type": "integer"},
									"macros": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"cal":     map[string]any{"type": "integer"},
											"fat":     map[string]any{"type": "integer"},
											"protein": map[string]any{"type": "integer"},
											"carbs":   map[string]any{"type": "integer"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Provider: optionalnullable.OptionalNullable[components.ProviderPreferences]{
			true: &preferences,
		},
	}, nil)
	if err != nil {
		return nil, apperrors.ErrLLMProvider
	}
	var response models.LabelResponse
	if len(res.ChatResult.Choices) == 0 {
		return nil, apperrors.ErrLLMProvider
	}
	content, ok := res.ChatResult.Choices[0].Message.Content.Get()
	if !ok || content == nil {
		return nil, apperrors.ErrLLMProvider
	}

	if content.Type != components.ChatAssistantMessageContentTypeStr || content.Str == nil {
		return nil, apperrors.ErrLLMProvider
	}
	responseText := *content.Str

	err = json.Unmarshal([]byte(responseText), &response)
	if err != nil {
		return nil, err
	}

	if !response.Success {
		return nil, apperrors.ErrNoLabelFound
	}
	return &response.Payload, nil

}
