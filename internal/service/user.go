package service

import (
	"main/internal/models"
)

type UserService struct {
	userStore models.UserRepository
}

func NewUserService(store models.UserRepository) *UserService {
	return &UserService{userStore: store}
}
