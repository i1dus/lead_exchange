package property

import (
	"context"
	"errors"
	"fmt"
	"lead_exchange/internal/domain"
	"lead_exchange/internal/lib/logger/sl"
	"lead_exchange/internal/lib/ml"
	"lead_exchange/internal/repository"
	"log/slog"

	"github.com/google/uuid"
)

type PropertyRepository interface {
	CreateProperty(ctx context.Context, property domain.Property) (uuid.UUID, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Property, error)
	UpdateProperty(ctx context.Context, propertyID uuid.UUID, update domain.PropertyFilter) error
	ListProperties(ctx context.Context, filter domain.PropertyFilter) ([]domain.Property, error)
	UpdateEmbedding(ctx context.Context, propertyID uuid.UUID, embedding []float32) error
	MatchProperties(ctx context.Context, leadEmbedding []float32, filter domain.PropertyFilter, limit int) ([]domain.MatchedProperty, error)
}

// LeadService нужен для получения embedding лида при матчинге.
type LeadService interface {
	GetLead(ctx context.Context, id uuid.UUID) (domain.Lead, error)
}

type Service struct {
	log        *slog.Logger
	repo       PropertyRepository
	mlClient   ml.Client
	leadService LeadService
}

var (
	ErrPropertyNotFound = errors.New("property not found")
)

func New(log *slog.Logger, repo PropertyRepository, mlClient ml.Client, leadService LeadService) *Service {
	return &Service{
		log:        log,
		repo:       repo,
		mlClient:   mlClient,
		leadService: leadService,
	}
}

// CreateProperty — создаёт новый объект недвижимости и генерирует embedding.
func (s *Service) CreateProperty(ctx context.Context, property domain.Property) (uuid.UUID, error) {
	const op = "property.Service.CreateProperty"
	log := s.log.With(slog.String("op", op), slog.String("title", property.Title))

	log.Info("creating new property")

	// Сначала сохраняем объект без embedding
	id, err := s.repo.CreateProperty(ctx, property)
	if err != nil {
		log.Error("failed to create property", sl.Err(err))
		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("property created successfully", slog.String("property_id", id.String()))

	// Генерируем embedding асинхронно (в фоне)
	go func() {
		if err := s.generateAndUpdateEmbedding(context.Background(), id, property); err != nil {
			s.log.Error("failed to generate embedding", slog.String("property_id", id.String()), sl.Err(err))
		}
	}()

	return id, nil
}

// generateAndUpdateEmbedding генерирует embedding для объекта недвижимости и обновляет запись.
func (s *Service) generateAndUpdateEmbedding(ctx context.Context, propertyID uuid.UUID, property domain.Property) error {
	const op = "property.Service.generateAndUpdateEmbedding"

	// Подготавливаем запрос к ML сервису
	mlReq := ml.PrepareAndEmbedRequest{
		Title:       property.Title,
		Description: property.Description,
		Price:       property.Price,
		Rooms:       property.Rooms,
		Area:        property.Area,
		Address:     &property.Address,
	}

	// Получаем embedding от ML сервиса
	mlResp, err := s.mlClient.PrepareAndEmbed(ctx, mlReq)
	if err != nil {
		return fmt.Errorf("%s: failed to get embedding: %w", op, err)
	}

	// Конвертируем []float64 в []float32
	embedding := make([]float32, len(mlResp.Embedding))
	for i, v := range mlResp.Embedding {
		embedding[i] = float32(v)
	}

	// Обновляем embedding в БД
	if err := s.repo.UpdateEmbedding(ctx, propertyID, embedding); err != nil {
		return fmt.Errorf("%s: failed to update embedding: %w", op, err)
	}

	s.log.Info("embedding generated and updated", slog.String("property_id", propertyID.String()))
	return nil
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

// MatchProperties находит подходящие объекты недвижимости для лида по векторному сходству.
func (s *Service) MatchProperties(ctx context.Context, leadID uuid.UUID, filter domain.PropertyFilter, limit int) ([]domain.MatchedProperty, error) {
	const op = "property.Service.MatchProperties"

	// Получаем лид с embedding
	lead, err := s.leadService.GetLead(ctx, leadID)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get lead: %w", op, err)
	}

	if len(lead.Embedding) == 0 {
		return nil, fmt.Errorf("%s: lead has no embedding", op)
	}

	if limit <= 0 {
		limit = 10 // Значение по умолчанию
	}

	matches, err := s.repo.MatchProperties(ctx, lead.Embedding, filter, limit)
	if err != nil {
		s.log.Error("failed to match properties", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return matches, nil
}

