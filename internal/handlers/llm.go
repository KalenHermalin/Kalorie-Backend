package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"main/internal/apperrors"
	"main/internal/service"
	"main/internal/utils"
	"net/http"
)

type LLMHandler struct {
	llmService service.LLMService
}

func NewLLMHandler(llm service.LLMService) *LLMHandler {
	return &LLMHandler{llmService: llm}
}

type analayzeRequestPayload struct {
	Picture []byte `json:"picture"`
}

func (handler *LLMHandler) AnalyzeFoodHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	var requestData analayzeRequestPayload
	if err := utils.DecodePayload(request.Body, &requestData); err != nil {
		// Bad Request because all we did was decode it and got an error meaning invalid JSON
		slog.Error("Error: decoding analyze food body", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInvalidRequest)
		return
	}
	// Retrieve request context to pass down
	ctx := request.Context()
	payload, err := handler.llmService.Provider.AnalyzePicture(ctx, requestData.Picture, utils.ANALYZEFOODSYSTEMPROMPT)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			apperrors.WriteError(writer, *appErr)
			return

		}
		slog.Error("Error: Analyze Food", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInternalServer)
		return
	}
	writer.WriteHeader(http.StatusOK)
	json.NewEncoder(writer).Encode(payload)
}

func (handler *LLMHandler) AnalyzeLabelHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	var requestData analayzeRequestPayload

	if err := utils.DecodePayload(request.Body, &requestData); err != nil {
		slog.Error("Error: decoding analyze label body", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInvalidRequest)
		return
	}
	ctx := request.Context()
	payload, err := handler.llmService.Provider.AnalyzeLabel(ctx, requestData.Picture, utils.ANALYZELABELSYSTEMPROMPT)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			apperrors.WriteError(writer, *appErr)
			return

		}
		slog.Error("Error: Analyze label", "error", err.Error())
		apperrors.WriteError(writer, *apperrors.ErrInternalServer)
		return
	}
	writer.WriteHeader(http.StatusOK)
	json.NewEncoder(writer).Encode(payload)
}
