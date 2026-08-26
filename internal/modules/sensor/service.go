package sensor

import (
	"context"

	responsehandler "capstone-be/internal/utils"
)

type SensorService interface {
	Create(ctx context.Context, req CreateSensorRequest) (*Sensor, error)
	Update(ctx context.Context, req UpdateSensorRequest) (*Sensor, error)
	Delete(ctx context.Context, req DeleteSensorRequest) error
	List(ctx context.Context, req ListSensorRequest) (*responsehandler.ListResponse[Sensor], error)
	Detail(ctx context.Context, req DetailSensorRequest) (*Sensor, error)
}

type sensorService struct {
	repo SensorRepository
}

func NewSensorService(repo SensorRepository) SensorService {
	return &sensorService{repo: repo}
}

func (s *sensorService) Create(ctx context.Context, req CreateSensorRequest) (*Sensor, error) {
	id, err := s.repo.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.repo.Detail(ctx, DetailSensorRequest{ID: id})
}

func (s *sensorService) Update(ctx context.Context, req UpdateSensorRequest) (*Sensor, error) {
	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return s.repo.Detail(ctx, DetailSensorRequest{ID: req.ID})
}

func (s *sensorService) Delete(ctx context.Context, req DeleteSensorRequest) error {
	return s.repo.Delete(ctx, req)
}

func (s *sensorService) List(ctx context.Context, req ListSensorRequest) (*responsehandler.ListResponse[Sensor], error) {
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

	return &responsehandler.ListResponse[Sensor]{
		List:  sensors,
		Count: count,
		Index: index,
		Limit: limit,
	}, nil
}

func (s *sensorService) Detail(ctx context.Context, req DetailSensorRequest) (*Sensor, error) {
	return s.repo.Detail(ctx, req)
}
