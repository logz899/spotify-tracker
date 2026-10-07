package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"spotify-backend/internal/models"
)

func TestRegisterRejectsPasswordShorterThanTenCharacters(t *testing.T) {
	handler := &Handler{}
	response := performHandlerRequest(http.MethodPost, "/register", `{"username":"listener","email":"listener@example.test","password":"123456789"}`, nil, handler.Register)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := decodeJSONBody(t, response)
	if _, exists := body["details"]; exists {
		t.Fatal("validation response exposed internal details")
	}
}

func TestRegisterDoesNotExposeDatabaseErrors(t *testing.T) {
	handler := &Handler{authStore: &fakeAuthStore{createErr: errors.New("sensitive SQL detail")}}
	response := performHandlerRequest(http.MethodPost, "/register", `{"username":"listener","email":"listener@example.test","password":"1234567890"}`, nil, handler.Register)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "sensitive SQL detail") {
		t.Fatal("response exposed database details")
	}
}

type fakeAuthStore struct {
	createErr error
}

func (f *fakeAuthStore) GetUserByUsername(context.Context, string) (*models.User, error) {
	return nil, errors.New("not found")
}

func (f *fakeAuthStore) GetUserByEmail(context.Context, string) (*models.User, error) {
	return nil, errors.New("not found")
}

func (f *fakeAuthStore) CreateUser(context.Context, *models.User) error {
	return f.createErr
}
