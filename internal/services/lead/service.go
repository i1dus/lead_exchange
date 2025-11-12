package lead

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

type LeadRepository interface {
	CreateLead(ctx context.Context, lead domain.Lead) (uuid.UUID, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Lead, error)
	UpdateLead(ctx context.Context, leadID uuid.UUID, update domain.LeadFilter) error
	ListLeads(ctx context.Context, filter domain.LeadFilter) ([]domain.Lead, error)
}

type Service struct {
	log  *slog.Logger
	repo LeadRepository
}

var (
	ErrLeadNotFound = errors.New("lead not found")
)

func New(log *slog.Logger, repo LeadRepository) *Service {
	return &Service{
		log:  log,
		repo: repo,
	}
}

// CreateLead — создаёт нового лида.
func (s *Service) CreateLead(ctx context.Context, lead domain.Lead) (uuid.UUID, error) {
	const op = "lead.Service.CreateLead"
	log := s.log.With(slog.String("op", op), slog.String("title", lead.Title))

	log.Info("creating new lead")

	id, err := s.repo.CreateLead(ctx, lead)
	if err != nil {
		log.Error("failed to create lead", sl.Err(err))
		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("lead created successfully", slog.String("lead_id", id.String()))
	return id, nil
}

// GetLead — получает лида по ID.
func (s *Service) GetLead(ctx context.Context, id uuid.UUID) (domain.Lead, error) {
	const op = "lead.Service.GetLead"

	lead, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrLeadNotFound) {
			s.log.Warn("lead not found", slog.String("lead_id", id.String()))
			return domain.Lead{}, fmt.Errorf("%s: %w", op, ErrLeadNotFound)
		}
		s.log.Error("failed to get lead", sl.Err(err))
		return domain.Lead{}, fmt.Errorf("%s: %w", op, err)
	}

	return lead, nil
}

// UpdateLead — частичное обновление данных лида.
func (s *Service) UpdateLead(ctx context.Context, leadID uuid.UUID, update domain.LeadFilter) (domain.Lead, error) {
	const op = "lead.Service.UpdateLead"

	err := s.repo.UpdateLead(ctx, leadID, update)
	if err != nil {
		if errors.Is(err, repository.ErrLeadNotFound) {
			return domain.Lead{}, fmt.Errorf("%s: %w", op, ErrLeadNotFound)
		}
		return domain.Lead{}, fmt.Errorf("%s: %w", op, err)
	}

	updated, err := s.repo.GetByID(ctx, leadID)
	if err != nil {
		return domain.Lead{}, fmt.Errorf("%s: failed to fetch updated lead: %w", op, err)
	}

	return updated, nil
}

// ListLeads — возвращает лидов по фильтру.
func (s *Service) ListLeads(ctx context.Context, filter domain.LeadFilter) ([]domain.Lead, error) {
	const op = "lead.Service.ListLeads"

	leads, err := s.repo.ListLeads(ctx, filter)
	if err != nil {
		s.log.Error("failed to list leads", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return leads, nil
}
