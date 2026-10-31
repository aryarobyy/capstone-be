package history

import "context"

type HistoryService interface {
	Create(ctx context.Context, req CreateHistoryRequest) (*HistoryResponse, error)
	List(ctx context.Context, req ListHistoryRequest) (*ListHistoryResponse, error)
	Newest(ctx context.Context, req NewestHistoryRequest) (*HistoryResponse, error)
	Detail(ctx context.Context, req DetailHistoryRequest) (*HistoryResponse, error)
	Delete(ctx context.Context, req DeleteHistoryRequest) error
	Since(ctx context.Context, req StreamHistoryRequest, afterID int64, limit int) ([]HistoryResponse, error)
}

type historyService struct {
	repo HistoryRepository
}

func NewHistoryService(repo HistoryRepository) HistoryService {
	return &historyService{repo: repo}
}

func (s *historyService) Create(ctx context.Context, req CreateHistoryRequest) (*HistoryResponse, error) {
	if !req.Status.IsValid() {
		return nil, ErrInvalidAnomalyStatus
	}

	if req.LastDetectedAt.IsZero() {
		req.LastDetectedAt = req.StartedAt
	}

	anomaly, err := s.repo.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	response := toHistoryResponse(anomaly)
	return &response, nil
}

func (s *historyService) List(ctx context.Context, req ListHistoryRequest) (*ListHistoryResponse, error) {
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

	histories, total, err := s.repo.List(ctx, HistoryFilter{
		OwnerID:   req.OwnerID,
		SensorID:  req.SensorID,
		Parameter: req.Parameter,
		Status:    req.Status,
		Date:      req.Date,
		Active:    req.Active,
		Limit:     limit,
		Index:     index,
	})
	if err != nil {
		return nil, err
	}

	data := make([]HistoryResponse, 0, len(histories))
	for _, anomaly := range histories {
		data = append(data, toHistoryResponse(&anomaly))
	}

	return &ListHistoryResponse{
		Data:  data,
		Total: total,
		Limit: limit,
		Index: index,
	}, nil
}

func (s *historyService) Newest(ctx context.Context, req NewestHistoryRequest) (*HistoryResponse, error) {
	anomaly, err := s.repo.Newest(ctx, req)
	if err != nil {
		return nil, err
	}

	response := toHistoryResponse(anomaly)
	return &response, nil
}

func (s *historyService) Detail(ctx context.Context, req DetailHistoryRequest) (*HistoryResponse, error) {
	anomaly, err := s.repo.Detail(ctx, req)
	if err != nil {
		return nil, err
	}

	response := toHistoryResponse(anomaly)
	return &response, nil
}

func (s *historyService) Delete(ctx context.Context, req DeleteHistoryRequest) error {
	return s.repo.Delete(ctx, req)
}

func (s *historyService) Since(ctx context.Context, req StreamHistoryRequest, afterID int64, limit int) ([]HistoryResponse, error) {
	histories, err := s.repo.Since(ctx, HistoryFilter{
		OwnerID:   req.OwnerID,
		SensorID:  req.SensorID,
		Parameter: req.Parameter,
		Status:    req.Status,
		Date:      req.Date,
		Limit:     limit,
	}, afterID)
	if err != nil {
		return nil, err
	}

	data := make([]HistoryResponse, 0, len(histories))
	for _, anomaly := range histories {
		data = append(data, toHistoryResponse(&anomaly))
	}
	return data, nil
}

func toHistoryResponse(anomaly *Anomaly) HistoryResponse {
	return HistoryResponse{
		ID:                  anomaly.ID,
		SensorID:            anomaly.SensorID,
		Parameter:           anomaly.Parameter,
		StartedAt:           anomaly.StartedAt,
		LastDetectedAt:      anomaly.LastDetectedAt,
		AccumulatedDuration: anomaly.AccumulatedDuration,
		Status:              anomaly.Status,
		ResolvedAt:          anomaly.ResolvedAt,
		CreatedAt:           anomaly.CreatedAt,
		UpdatedAt:           anomaly.UpdatedAt,
	}
}
