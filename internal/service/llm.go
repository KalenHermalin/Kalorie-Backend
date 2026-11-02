package service

import (
	"context"
	"errors"
	"fmt"
	"main/internal/apperrors"
	"main/internal/llm"
	"main/internal/models"
	"main/internal/utils"
)

type LLMService struct {
	Provider models.LLMProvider
}

func NewLLMService(provider models.LLMProvider) *LLMService {
	return &LLMService{Provider: provider}
}

func (s *LLMService) AnalyzeFood(ctx context.Context, requestData *models.AnalayzeFoodRequestPayload) (payload *models.MealPayload, err error) {
	if requestData.Picture == nil {
		return nil, apperrors.New(apperrors.BadRequest, fmt.Errorf("no picture submitted"))
	}
	pctx, cancel := context.WithTimeout(ctx, s.Provider.GetTimeoutTime())
	payload, err = s.Provider.AnalyzePicture(pctx, requestData.Picture, requestData.Description, utils.ANALYZEFOODSYSTEMPROMPT)
	cancel()
	if err == nil {
		return payload, nil
	}
	if errors.Is(err, llm.NoFoodFound) {
		return nil, apperrors.New(apperrors.NoFoodFound, err)
	}
	if errors.Is(err, llm.ProviderError) {
		return nil, apperrors.New(apperrors.LlmProviderDown, err)
	}

	return nil, apperrors.New(apperrors.Internal, err)

}

func (s *LLMService) AnalyzeLabel(ctx context.Context, requestData *models.AnalayzeLabelRequestPayload) (payload *models.LabelPayload, err error) {
	if requestData.Picture == nil {
		return nil, apperrors.New(apperrors.BadRequest, fmt.Errorf("no picture submitted"))
	}
	pctx, cancel := context.WithTimeout(ctx, s.Provider.GetTimeoutTime())
	payload, err = s.Provider.AnalyzeLabel(pctx, requestData.Picture, utils.ANALYZELABELSYSTEMPROMPT)
	cancel()
	if err == nil {
		return payload, nil
	}
	if errors.Is(err, llm.NoLabelFound) {
		return nil, apperrors.New(apperrors.NoLabelFound, err)
	}
	if errors.Is(err, llm.ProviderError) {
		return nil, apperrors.New(apperrors.LlmProviderDown, err)

	}

	return nil, apperrors.New(apperrors.Internal, err)

}
