package service

import (
	"context"
	"errors"
	"log/slog"
	"main/internal/apperrors"
	"main/internal/models"
	"main/internal/utils"
)

type LLMService struct {
	Providers []models.LLMProvider
}

func NewLLMService(providers ...models.LLMProvider) *LLMService {
	return &LLMService{Providers: providers}
}

func (s *LLMService) AnalyzeFood(ctx context.Context, requestData *models.AnalayzeFoodRequestPayload) (*models.MealPayload, error) {

	for _, provider := range s.Providers {
		pctx, cancel := context.WithTimeout(ctx, provider.GetTimeoutTime())
		payload, err := provider.AnalyzePicture(pctx, requestData.Picture, requestData.Description, utils.ANALYZEFOODSYSTEMPROMPT)
		cancel()
		if err == nil {
			return payload, nil
		}
		if errors.Is(err, apperrors.ErrNoFoodFound) {
			return nil, err
		}
		if errors.Is(err, apperrors.ErrLLMProvider) {
			continue
		}
		slog.Error("Error:" + err.Error())
		break

	}
	return nil, apperrors.ErrInternalServer

}

func (s *LLMService) AnalyzeLabel(ctx context.Context, requestData *models.AnalayzeLabelRequestPayload) (*models.LabelPayload, error) {

	for _, provider := range s.Providers {
		pctx, cancel := context.WithTimeout(ctx, provider.GetTimeoutTime())
		payload, err := provider.AnalyzeLabel(pctx, requestData.Picture, utils.ANALYZELABELSYSTEMPROMPT)
		cancel()
		if err == nil {
			return payload, nil
		}
		if errors.Is(err, apperrors.ErrNoLabelFound) {
			return nil, err
		}
		if errors.Is(err, apperrors.ErrLLMProvider) {
			continue
		}
		slog.Error("Error:" + err.Error())
		break

	}
	return nil, apperrors.ErrInternalServer

}
