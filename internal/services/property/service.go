package property

import (
	"context"
	"errors"
	"fmt"
	"lead_exchange/internal/domain"
	"lead_exchange/internal/lib/logger/sl"
	"lead_exchange/internal/repository"
	"log/slog"

	"github.com/google/uuid"
)

type PropertyRepository interface {
	CreateProperty(ctx context.Context, property domain.Property) (uuid.UUID, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Property, error)
	UpdateProperty(ctx context.Context, propertyID uuid.UUID, update domain.PropertyFilter) error
	ListProperties(ctx context.Context, filter domain.PropertyFilter) ([]domain.Property, error)
}

type Service struct {
	log  *slog.Logger
	repo PropertyRepository
}

var (
	ErrPropertyNotFound = errors.New("property not found")
)

func New(log *slog.Logger, repo PropertyRepository) *Service {
	return &Service{
		log:  log,
		repo: repo,
	}
}

// CreateProperty — создаёт новый объект недвижимости.
func (s *Service) CreateProperty(ctx context.Context, property domain.Property) (uuid.UUID, error) {
	const op = "property.Service.CreateProperty"
	log := s.log.With(slog.String("op", op), slog.String("title", property.Title))

	log.Info("creating new property")

	id, err := s.repo.CreateProperty(ctx, property)
	if err != nil {
		log.Error("failed to create property", sl.Err(err))
		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("property created successfully", slog.String("property_id", id.String()))
	return id, nil
}

// GetProperty — получает объект недвижимости по ID.
func (s *Service) GetProperty(ctx context.Context, id uuid.UUID) (domain.Property, error) {
	const op = "property.Service.GetProperty"

	property, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrPropertyNotFound) {
			s.log.Warn("property not found", slog.String("property_id", id.String()))
			return domain.Property{}, fmt.Errorf("%s: %w", op, ErrPropertyNotFound)
		}
		s.log.Error("failed to get property", sl.Err(err))
		return domain.Property{}, fmt.Errorf("%s: %w", op, err)
	}

	return property, nil
}

// UpdateProperty — частичное обновление данных объекта недвижимости.
func (s *Service) UpdateProperty(ctx context.Context, propertyID uuid.UUID, update domain.PropertyFilter) (domain.Property, error) {
	const op = "property.Service.UpdateProperty"

	err := s.repo.UpdateProperty(ctx, propertyID, update)
	if err != nil {
		if errors.Is(err, repository.ErrPropertyNotFound) {
			return domain.Property{}, fmt.Errorf("%s: %w", op, ErrPropertyNotFound)
		}
		return domain.Property{}, fmt.Errorf("%s: %w", op, err)
	}

	updated, err := s.repo.GetByID(ctx, propertyID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("%s: failed to fetch updated property: %w", op, err)
	}

	return updated, nil
}

// ListProperties — возвращает объекты недвижимости по фильтру.
func (s *Service) ListProperties(ctx context.Context, filter domain.PropertyFilter) ([]domain.Property, error) {
	const op = "property.Service.ListProperties"

	properties, err := s.repo.ListProperties(ctx, filter)
	if err != nil {
		s.log.Error("failed to list properties", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return properties, nil
}

