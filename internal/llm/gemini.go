package llm

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"main/internal/models"
	"strings"

	"google.golang.org/genai"
)

type GeminiProvider struct {
	client *genai.Client
	model  string
}

type geminiMealResponse struct {
	Success bool               `json:"success"`
	Payload models.MealPayload `json:"payload"`
}

func NewGeminiProvider(ctx context.Context, apiKey string, model string) (*GeminiProvider, error) {

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	it, err := client.Models.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	for {
		m, err := it.Next(ctx)
		if err != nil {
			break
		}
		slog.Info("Available Model", "name", m.Name)
	}
	if err != nil {
		return nil, err
	}

	return &GeminiProvider{client: client, model: model}, nil
}

func (gm *GeminiProvider) AnalyzePicture(ctx context.Context, picture []byte, systemPrompt string) (*models.MealPayload, error) {

	userParts := []*genai.Part{
		{InlineData: &genai.Blob{Data: picture, MIMEType: "image/jpeg"}},
	}
	var systemParts []*genai.Part
	if len(strings.TrimSpace(systemPrompt)) > 0 {
		systemParts = []*genai.Part{
			{Text: systemPrompt},
		}

	}

	result, err := gm.client.Models.GenerateContent(context.Background(), gm.model, []*genai.Content{{Parts: userParts}}, &genai.GenerateContentConfig{
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
		return nil, err
	}
	var geminiResponse geminiMealResponse
	err = json.Unmarshal([]byte(result.Text()), &geminiResponse)
	if err != nil {
		return nil, err
	}

	if !geminiResponse.Success {
		return nil, errors.New("No food found!")
	}
	return &geminiResponse.Payload, nil

}

type geminiLabelResponse struct {
	Success bool                `json:"success"`
	Payload models.LabelPayload `json:"payload"`
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
	result, err := gm.client.Models.GenerateContent(context.Background(), gm.model, []*genai.Content{{Parts: userParts}}, &genai.GenerateContentConfig{
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
		return nil, err
	}
	var geminiResponse geminiLabelResponse
	err = json.Unmarshal([]byte(result.Text()), &geminiResponse)
	if err != nil {
		return nil, err
	}
	if !geminiResponse.Success {
		return nil, errors.New("No label found in image")
	}
	return &geminiResponse.Payload, nil

}
