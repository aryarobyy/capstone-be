package sensor

import (
	"context"
	"errors"

	"capstone-be/internal/middleware"
	"capstone-be/internal/modules/area"
)

var (
	ErrAreaNotFound = errors.New("area not found")
)

type SensorService interface {
	Create(ctx context.Context, req CreateSensorRequest) (*SensorResponse, error)
	Update(ctx context.Context, req UpdateSensorRequest) (*SensorResponse, error)
	Delete(ctx context.Context, req DeleteSensorRequest) error
	List(ctx context.Context, req ListSensorRequest) (*ListSensorResponse, error)
	Detail(ctx context.Context, req DetailSensorRequest) (*SensorResponse, error)
}

type sensorService struct {
	repo     SensorRepository
	areaRepo area.AreaRepository
}

func NewSensorService(repo SensorRepository, areaRepo area.AreaRepository) SensorService {
	return &sensorService{repo: repo, areaRepo: areaRepo}
}

func toSensorResponse(s *Sensor) *SensorResponse {
	if s == nil {
		return nil
	}
	return &SensorResponse{
		ID:          s.ID,
		AreaID:      s.AreaID,
		OwnerID:     s.OwnerID,
		Name:        s.Name,
		Code:        s.Code,
		Description: s.Description,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func (s *sensorService) Create(ctx context.Context, req CreateSensorRequest) (*SensorResponse, error) {
	if req.OwnerID == 0 {
		if uid, ok := middleware.GetUserIDFromContext(ctx); ok {
			req.OwnerID = uid
		}
	}

	if req.AreaID != nil && *req.AreaID != 0 {
		exists, err := s.areaRepo.Exists(ctx, *req.AreaID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrAreaNotFound
		}
	}

	id, err := s.repo.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	sensor, err := s.repo.Detail(ctx, DetailSensorRequest{ID: id, OwnerID: req.OwnerID})
	if err != nil {
		return nil, err
	}
	return toSensorResponse(sensor), nil
}

func (s *sensorService) Update(ctx context.Context, req UpdateSensorRequest) (*SensorResponse, error) {
	if req.OwnerID == 0 {
		if uid, ok := middleware.GetUserIDFromContext(ctx); ok {
			req.OwnerID = uid
		}
	}

	if req.AreaID != nil && *req.AreaID != 0 {
		exists, err := s.areaRepo.Exists(ctx, *req.AreaID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrAreaNotFound
		}
	}

	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	sensor, err := s.repo.Detail(ctx, DetailSensorRequest{ID: req.ID, OwnerID: req.OwnerID})
	if err != nil {
		return nil, err
	}
	return toSensorResponse(sensor), nil
}

func (s *sensorService) Delete(ctx context.Context, req DeleteSensorRequest) error {
	if req.OwnerID == 0 {
		if uid, ok := middleware.GetUserIDFromContext(ctx); ok {
			req.OwnerID = uid
		}
	}
	return s.repo.Delete(ctx, req)
}

func (s *sensorService) List(ctx context.Context, req ListSensorRequest) (*ListSensorResponse, error) {
	if req.OwnerID == 0 {
		if uid, ok := middleware.GetUserIDFromContext(ctx); ok {
			req.OwnerID = uid
		}
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	index := req.Index
	if index < 0 {
		index = 0
	}

	sensors, count, err := s.repo.List(ctx, SensorFilter{
		OwnerID: req.OwnerID,
		AreaID:  req.AreaID,
		Name:    req.Name,
		Limit:   limit,
		Index:   index,
	})
	if err != nil {
		return nil, err
	}

	data := make([]ListSensorData, 0, len(sensors))
	for _, item := range sensors {
		data = append(data, ListSensorData{
			ID:          item.ID,
			AreaID:      item.AreaID,
			OwnerID:     item.OwnerID,
			Name:        item.Name,
			Code:        item.Code,
			Description: item.Description,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	return &ListSensorResponse{
		Data:  data,
		Total: count,
		Limit: limit,
		Index: index,
	}, nil
}

func (s *sensorService) Detail(ctx context.Context, req DetailSensorRequest) (*SensorResponse, error) {
	if req.OwnerID == 0 {
		if uid, ok := middleware.GetUserIDFromContext(ctx); ok {
			req.OwnerID = uid
		}
	}

	sensor, err := s.repo.Detail(ctx, req)
	if err != nil {
		return nil, err
	}
	return toSensorResponse(sensor), nil
}
