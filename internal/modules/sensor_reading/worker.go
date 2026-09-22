package sensorreading

import (
	"context"
	"log"
	"sync"
	"time"
)

type SummaryCache struct {
	mu    sync.RWMutex
	items map[int64]SensorSummaryItem
}

var GlobalSummaryCache = &SummaryCache{
	items: make(map[int64]SensorSummaryItem),
}

func (c *SummaryCache) Set(item SensorSummaryItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[item.SensorID] = item
}

func (c *SummaryCache) SetBulk(items []SensorSummaryItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, item := range items {
		c.items[item.SensorID] = item
	}
}

func (c *SummaryCache) Get(sensorID int64) (SensorSummaryItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[sensorID]
	return item, ok
}

func (c *SummaryCache) GetAll() []SensorSummaryItem {
	c.mu.RLock()
	defer c.mu.RUnlock()
	list := make([]SensorSummaryItem, 0, len(c.items))
	for _, item := range c.items {
		list = append(list, item)
	}
	return list
}

// PeriodicCalcWorker periodically aggregates sensor reading telemetry into memory cache.
type PeriodicCalcWorker struct {
	repo          SensorReadingRepository
	interval      time.Duration
	windowMinutes int
}

func NewPeriodicCalcWorker(repo SensorReadingRepository, interval time.Duration, windowMinutes int) *PeriodicCalcWorker {
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	if windowMinutes <= 0 {
		windowMinutes = 10
	}
	return &PeriodicCalcWorker{
		repo:          repo,
		interval:      interval,
		windowMinutes: windowMinutes,
	}
}

func (w *PeriodicCalcWorker) Run(ctx context.Context) {
	log.Printf("[SensorReadingWorker] Started periodic calculation worker (interval: %v, window: %dm)", w.interval, w.windowMinutes)
	
	// Run initial calculation immediately on startup
	w.calculate(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[SensorReadingWorker] Stopping periodic calculation worker...")
			return
		case <-ticker.C:
			w.calculate(ctx)
		}
	}
}

func (w *PeriodicCalcWorker) calculate(ctx context.Context) {
	calcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	summaries, err := w.repo.CalculateSummary(calcCtx, 0, 0, w.windowMinutes)
	if err != nil {
		log.Printf("[SensorReadingWorker] Error calculating sensor summaries: %v", err)
		return
	}

	GlobalSummaryCache.SetBulk(summaries)
	log.Printf("[SensorReadingWorker] Updated summary cache for %d sensors", len(summaries))
}
