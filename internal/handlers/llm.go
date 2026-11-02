package handlers

import (
	"context"
	"encoding/json"
	"main/internal/apperrors"
	"main/internal/models"
	"main/internal/service"
	"main/internal/utils"
	"net/http"
	"time"
)

type LLMHandler struct {
	llmService service.LLMService
}

func NewLLMHandler(llm service.LLMService) *LLMHandler {
	return &LLMHandler{llmService: llm}
}

func (handler *LLMHandler) AnalyzeFoodHandler(writer http.ResponseWriter, request *http.Request) error {
	writer.Header().Set("Content-Type", "application/json")

	var requestData models.AnalayzeFoodRequestPayload
	if err := utils.DecodePayload(request.Body, &requestData); err != nil {
		// Bad Request because all we did was decode it and got an error meaning invalid JSON
		return apperrors.New(apperrors.BadRequest, err)
	}
	// Retrieve request context to pass down
	ctx := request.Context()
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	payload, err := handler.llmService.AnalyzeFood(ctx, &requestData)
	if err != nil {
		return err
	}
	writer.WriteHeader(http.StatusOK)
	json.NewEncoder(writer).Encode(payload)
	return nil
}

func (handler *LLMHandler) AnalyzeLabelHandler(writer http.ResponseWriter, request *http.Request) error {
	writer.Header().Set("Content-Type", "application/json")

	var requestData models.AnalayzeLabelRequestPayload

	if err := utils.DecodePayload(request.Body, &requestData); err != nil {
		return apperrors.New(apperrors.BadRequest, err)

	}
	ctx := request.Context()
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	payload, err := handler.llmService.AnalyzeLabel(ctx, &requestData)
	if err != nil {
		return err
	}
	writer.WriteHeader(http.StatusOK)
	json.NewEncoder(writer).Encode(payload)
	return nil
}
