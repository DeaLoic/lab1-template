package person

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound — запись с таким id отсутствует.
var ErrNotFound = errors.New("person not found")

// ValidationError — ошибки валидации запроса: поле → описание.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %v", e.Fields)
}

// Repository — хранилище записей о людях.
type Repository interface {
	Create(ctx context.Context, p Person) (int32, error)
	GetByID(ctx context.Context, id int32) (Person, error)
	List(ctx context.Context) ([]Person, error)
	Update(ctx context.Context, id int32, p Person) (Person, error)
	Delete(ctx context.Context, id int32) error
	Close()
}

// Service — бизнес-логика над сущностью Person.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create сохраняет нового человека и возвращает его id.
func (s *Service) Create(ctx context.Context, req Request) (int32, error) {
	p, err := applyRequest(Person{}, req, true)
	if err != nil {
		return 0, err
	}
	return s.repo.Create(ctx, p)
}

// Get возвращает человека по id или ErrNotFound.
func (s *Service) Get(ctx context.Context, id int32) (Response, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Response{}, err
	}
	return toResponse(p), nil
}

// List возвращает всех людей.
func (s *Service) List(ctx context.Context) ([]Response, error) {
	people, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Response, 0, len(people))
	for _, p := range people {
		out = append(out, toResponse(p))
	}
	return out, nil
}

// Patch частично обновляет запись: применяются только заполненные поля запроса.
func (s *Service) Patch(ctx context.Context, id int32, req Request) (Response, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Response{}, err
	}
	updated, err := applyRequest(existing, req, false)
	if err != nil {
		return Response{}, err
	}
	saved, err := s.repo.Update(ctx, id, updated)
	if err != nil {
		return Response{}, err
	}
	return toResponse(saved), nil
}

// Delete удаляет запись по id или возвращает ErrNotFound.
func (s *Service) Delete(ctx context.Context, id int32) error {
	return s.repo.Delete(ctx, id)
}

// applyRequest накладывает запрос на запись. Для create поле name обязательно,
// для patch отсутствующие поля сохраняют прежние значения.
func applyRequest(p Person, req Request, create bool) (Person, error) {
	res := p

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return res, &ValidationError{Fields: map[string]string{"name": "must not be empty"}}
		}
		res.Name = name
	} else if create {
		return res, &ValidationError{Fields: map[string]string{"name": "is required"}}
	}

	if req.Age != nil {
		if *req.Age < 0 || *req.Age > 150 {
			return res, &ValidationError{Fields: map[string]string{"age": "must be between 0 and 150"}}
		}
		res.Age = req.Age
	}
	if req.Address != nil {
		res.Address = req.Address
	}
	if req.Work != nil {
		res.Work = req.Work
	}
	return res, nil
}
