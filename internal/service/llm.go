package service

import "main/internal/models"

type LLMService struct {
	Provider models.LLMProvider
}

func NewLLMService(provider models.LLMProvider) *LLMService {
	return &LLMService{Provider: provider}
}
