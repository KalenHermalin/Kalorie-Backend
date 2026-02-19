package handlers

import (
	"encoding/json"
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
		http.Error(writer, err.Error(), http.StatusBadRequest)
	}
	// Retrieve request context to pass down
	ctx := request.Context()
	payload, err := handler.llmService.Provider.AnalyzePicture(ctx, requestData.Picture, utils.ANALYZEFOODSYSTEMPROMPT)
	if err != nil {
		http.Error(writer, "Error in serivce", http.StatusBadRequest)
	}
	writer.WriteHeader(http.StatusOK)
	json.NewEncoder(writer).Encode(payload)
}

func (handler *LLMHandler) AnalyzeLabelHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	var requestData analayzeRequestPayload

	if err := utils.DecodePayload(request.Body, &requestData); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
	ctx := request.Context()
	payload, err := handler.llmService.Provider.AnalyzeLabel(ctx, requestData.Picture, utils.ANALYZELABELSYSTEMPROMPT)
	if err != nil {
		http.Error(writer, "Error in service", http.StatusBadRequest)
	}
	writer.WriteHeader(http.StatusOK)
	json.NewEncoder(writer).Encode(payload)
}
