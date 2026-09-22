package sensorreading

import (
	"context"
)

type SensorReadingService interface {
	Create(ctx context.Context, req CreateSensorReadingRequest) error
	List(ctx context.Context, req ListSensorReadingRequest) (*ListSensorReadingResponse, error)
	Detail(ctx context.Context, req DetailSensorReadingRequest) (*SensorReadingResponse, error)
	Delete(ctx context.Context, req DeleteSensorReadingRequest) error
	Summary(ctx context.Context, req SensorSummaryRequest, ownerID int64) (*SensorSummaryResponse, error)
}

type sensorReadingService struct {
	repo SensorReadingRepository
}

func NewSensorReadingService(repo SensorReadingRepository) SensorReadingService {
	return &sensorReadingService{repo: repo}
}

func (s *sensorReadingService) Create(ctx context.Context, req CreateSensorReadingRequest) error {
	return s.repo.Create(ctx, req)
}

func (s *sensorReadingService) List(ctx context.Context, req ListSensorReadingRequest) (*ListSensorReadingResponse, error) {
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

	filter := SensorReadingFilter{
		SensorID: req.SensorID,
		Date:     req.Date,
		Limit:    limit,
		Index:    index,
	}

	readings, totalCount, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	data := make([]SensorReadingResponse, 0, len(readings))
	for _, reading := range readings {
		data = append(data, SensorReadingResponse{
			ID:           reading.ID,
			SensorID:     reading.SensorID,
			SoilMoisture: reading.SoilMoisture,
			Temperature:  reading.Temperature,
			Humidity:     reading.Humidity,
			RecordedAt:   reading.RecordedAt,
		})
	}

	return &ListSensorReadingResponse{
		Data:  data,
		Total: totalCount,
		Limit: limit,
		Index: index,
	}, nil
}

func (s *sensorReadingService) Detail(ctx context.Context, req DetailSensorReadingRequest) (*SensorReadingResponse, error) {
	reading, err := s.repo.Detail(ctx, req)
	if err != nil {
		return nil, err
	}

	return &SensorReadingResponse{
		ID:           reading.ID,
		SensorID:     reading.SensorID,
		SoilMoisture: reading.SoilMoisture,
		Temperature:  reading.Temperature,
		Humidity:     reading.Humidity,
		RecordedAt:   reading.RecordedAt,
	}, nil
}

func (s *sensorReadingService) Delete(ctx context.Context, req DeleteSensorReadingRequest) error {
	return s.repo.Delete(ctx, req)
}

func (s *sensorReadingService) Summary(ctx context.Context, req SensorSummaryRequest, ownerID int64) (*SensorSummaryResponse, error) {
	windowMinutes := req.WindowMinutes
	if windowMinutes <= 0 {
		windowMinutes = 10
	}

	// 1. Check in-memory cache if window is default 10 minutes and specific sensor is requested
	if windowMinutes == 10 && req.SensorID > 0 {
		if cached, ok := GlobalSummaryCache.Get(req.SensorID); ok {
			return &SensorSummaryResponse{
				WindowMinutes: windowMinutes,
				Data:          []SensorSummaryItem{cached},
			}, nil
		}
	}

	// 2. Fetch fresh calculation from repository
	data, err := s.repo.CalculateSummary(ctx, req.SensorID, ownerID, windowMinutes)
	if err != nil {
		return nil, err
	}

	// Update cache with fresh data
	if windowMinutes == 10 && len(data) > 0 {
		GlobalSummaryCache.SetBulk(data)
	}

	return &SensorSummaryResponse{
		WindowMinutes: windowMinutes,
		Data:          data,
	}, nil
}

