package models

import "context"

type LabelPayload struct {
	Units        string `json:"units"`
	BaseQuantity uint16 `json:"base_quantity"`
	Macros       Macros `json:"macros"`
}
type Macros struct {
	Cal     uint16 `json:"cal"`
	Fat     uint16 `json:"fat"`
	Protein uint16 `json:"protein"`
	Carbs   uint16 `json:"carbs"`
}

type MealPayload struct {
	FoodName string `json:"food_name"`
	Cal      uint16 `json:"cal"`
	Fat      uint16 `json:"fat"`
	Protein  uint16 `json:"protein"`
	Carbs    uint16 `json:"carbs"`
}
type LLMProvider interface {
	AnalyzeLabel(ctx context.Context, picture []byte, systemPrompt string) (*LabelPayload, error)
	AnalyzePicture(ctx context.Context, picture []byte, systemPrompt string) (*MealPayload, error)
}
