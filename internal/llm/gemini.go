package llm

import (
	"context"
	"encoding/json"
	"main/internal/apperrors"
	"main/internal/models"
	"strings"
	"time"

	"google.golang.org/genai"
)

type GeminiProvider struct {
	client *genai.Client
	model  string
}

func NewGeminiProvider(ctx context.Context, apiKey string, model string) (*GeminiProvider, error) {

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}
	return &GeminiProvider{client: client, model: model}, nil
}
func (gm *GeminiProvider) GetTimeoutTime() time.Duration {
	return time.Second * 15
}
func (gm *GeminiProvider) AnalyzePicture(ctx context.Context, picture []byte, description, systemPrompt string) (*models.MealPayload, error) {
	//TODO: Seperate concerns, meaning build the genAI parts in sub function and return?
	userParts := []*genai.Part{
		{InlineData: &genai.Blob{Data: picture, MIMEType: "image/jpeg"}},
	}
	if len(strings.TrimSpace(description)) > 0 {
		userParts = append(userParts, &genai.Part{Text: description})

	}
	var systemParts []*genai.Part
	if len(strings.TrimSpace(systemPrompt)) > 0 {
		systemParts = []*genai.Part{
			{Text: systemPrompt},
		}

	}

	result, err := gm.client.Models.GenerateContent(ctx, gm.model, []*genai.Content{{Parts: userParts}}, &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: systemParts},
		ResponseMIMEType:  `application/json`,
		ResponseSchema: &genai.Schema{
			PropertyOrdering: []string{"success", "payload"},
			Type:             genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"success": {Type: genai.TypeBoolean},
				"payload": {
					Type:             genai.TypeObject,
					PropertyOrdering: []string{"food_name", "cal", "fat", "protein", "carbs"},
					Properties: map[string]*genai.Schema{
						"food_name": {Type: genai.TypeString, Description: "Simple name for the meal captured"},
						"cal":       {Type: genai.TypeInteger},
						"fat":       {Type: genai.TypeInteger},
						"protein":   {Type: genai.TypeInteger},
						"carbs":     {Type: genai.TypeInteger},
					},
				},
			},
		},
	})

	if err != nil {
		return nil, apperrors.ErrLLMProvider
	}
	var geminiResponse models.MealResponse
	err = json.Unmarshal([]byte(result.Text()), &geminiResponse)
	if err != nil {
		return nil, err
	}

	if !geminiResponse.Success {
		return nil, apperrors.ErrNoFoodFound
	}
	return &geminiResponse.Payload, nil

}

func (gm *GeminiProvider) AnalyzeLabel(ctx context.Context, picture []byte, systemPrompt string) (*models.LabelPayload, error) {

	userParts := []*genai.Part{
		{InlineData: &genai.Blob{Data: picture, MIMEType: "image/jpeg"}},
	}
	var systemParts []*genai.Part
	if len(strings.TrimSpace(systemPrompt)) > 0 {
		systemParts = []*genai.Part{
			{Text: systemPrompt},
		}

	}
	result, err := gm.client.Models.GenerateContent(ctx, gm.model, []*genai.Content{{Parts: userParts}}, &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: systemParts},
		ResponseMIMEType:  `application/json`,
		ResponseSchema: &genai.Schema{
			Type:             genai.TypeObject,
			PropertyOrdering: []string{"success", "payload"},
			Properties: map[string]*genai.Schema{
				"success": {Type: genai.TypeBoolean},
				"payload": {
					Type:             genai.TypeObject,
					PropertyOrdering: []string{"units", "base_quantity", "macros"},
					Properties: map[string]*genai.Schema{
						"units":         {Type: genai.TypeString},
						"base_quantity": {Type: genai.TypeInteger},
						"macros": {
							Type:             genai.TypeObject,
							PropertyOrdering: []string{"cal", "fat", "protein", "carbs"},
							Properties: map[string]*genai.Schema{
								"cal":     {Type: genai.TypeInteger},
								"fat":     {Type: genai.TypeInteger},
								"protein": {Type: genai.TypeInteger},
								"carbs":   {Type: genai.TypeInteger},
							},
						},
					},
				},
			},
		},
	})

	if err != nil {
		return nil, apperrors.ErrLLMProvider
	}
	var geminiResponse models.LabelResponse
	err = json.Unmarshal([]byte(result.Text()), &geminiResponse)
	if err != nil {
		return nil, err
	}
	if !geminiResponse.Success {
		return nil, apperrors.ErrNoLabelFound
	}
	return &geminiResponse.Payload, nil

}
