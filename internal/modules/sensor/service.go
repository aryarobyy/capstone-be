package sensor

import (
	"context"
)

type SensorService interface {
	Create(ctx context.Context, req CreateSensorRequest) (*SensorResponse, error)
	Update(ctx context.Context, req UpdateSensorRequest) (*SensorResponse, error)
	Delete(ctx context.Context, req DeleteSensorRequest) error
	List(ctx context.Context, req ListSensorRequest) (*ListSensorResponse, error)
	Detail(ctx context.Context, req DetailSensorRequest) (*SensorResponse, error)
}

type sensorService struct {
	repo SensorRepository
}

func NewSensorService(repo SensorRepository) SensorService {
	return &sensorService{repo: repo}
}

func toSensorResponse(s *Sensor) *SensorResponse {
	if s == nil {
		return nil
	}
	return &SensorResponse{
		ID:          s.ID,
		AreaID:      s.AreaID,
		Name:        s.Name,
		Type:        s.Type,
		Code:        s.Code,
		Description: s.Description,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func (s *sensorService) Create(ctx context.Context, req CreateSensorRequest) (*SensorResponse, error) {
	id, err := s.repo.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	sensor, err := s.repo.Detail(ctx, DetailSensorRequest{ID: id})
	if err != nil {
		return nil, err
	}
	return toSensorResponse(sensor), nil
}

func (s *sensorService) Update(ctx context.Context, req UpdateSensorRequest) (*SensorResponse, error) {
	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	sensor, err := s.repo.Detail(ctx, DetailSensorRequest{ID: req.ID})
	if err != nil {
		return nil, err
	}
	return toSensorResponse(sensor), nil
}

func (s *sensorService) Delete(ctx context.Context, req DeleteSensorRequest) error {
	return s.repo.Delete(ctx, req)
}

func (s *sensorService) List(ctx context.Context, req ListSensorRequest) (*ListSensorResponse, error) {
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
		AreaID: req.AreaID,
		Name:   req.Name,
		Type:   req.Type,
		Limit:  limit,
		Index:  index,
	})
	if err != nil {
		return nil, err
	}

	data := make([]ListSensorData, 0, len(sensors))
	for _, item := range sensors {
		data = append(data, ListSensorData{
			ID:          item.ID,
			AreaID:      item.AreaID,
			Name:        item.Name,
			Type:        item.Type,
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
	sensor, err := s.repo.Detail(ctx, req)
	if err != nil {
		return nil, err
	}
	return toSensorResponse(sensor), nil
}
