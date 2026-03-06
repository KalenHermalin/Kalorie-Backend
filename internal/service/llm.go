package service

import "main/internal/models"

type LLMService struct {
	Provider models.LLMProvider
}

func NewLLMService(provider models.LLMProvider) *LLMService {
	return &LLMService{Provider: provider}
}

//TODO: Create default funciton which then calls the provider
// Single analyze photo function which will work for meals and labels?
