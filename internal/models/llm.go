package models

import (
	"context"
	"time"
)

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

type MealResponse struct {
	Success bool        `json:"success"`
	Payload MealPayload `json:"payload"`
}
type LabelResponse struct {
	Success bool         `json:"success"`
	Payload LabelPayload `json:"payload"`
}
type LLMProvider interface {
	AnalyzeLabel(ctx context.Context, picture []byte, systemPrompt string) (*LabelPayload, error)
	AnalyzePicture(ctx context.Context, picture []byte, description, systemPrompt string) (*MealPayload, error)
	GetTimeoutTime() time.Duration
}

type AnalayzeFoodRequestPayload struct {
	Picture     []byte `json:"picture"`
	Description string `json:"description"`
}
type AnalayzeLabelRequestPayload struct {
	Picture []byte `json:"picture"`
}
