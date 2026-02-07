package valueobject

import (
	"errors"
	"time"
)

// TimeRange представляет временной диапазон (Value Object)
// Иммутабельный объект
type TimeRange struct {
	start time.Time
	end   time.Time
}

// NewTimeRange создает новый TimeRange с валидацией
func NewTimeRange(start, end time.Time) (TimeRange, error) {
	if start.After(end) {
		return TimeRange{}, errors.New("start time must be before end time")
	}

	if start.IsZero() || end.IsZero() {
		return TimeRange{}, errors.New("start and end times cannot be zero")
	}

	return TimeRange{
		start: start,
		end:   end,
	}, nil
}

// NewTimeRangeFromDuration создает TimeRange от указанного времени назад до текущего момента
func NewTimeRangeFromDuration(duration time.Duration) (TimeRange, error) {
	if duration <= 0 {
		return TimeRange{}, errors.New("duration must be positive")
	}

	now := time.Now()
	start := now.Add(-duration)

	return TimeRange{
		start: start,
		end:   now,
	}, nil
}

// Start возвращает начальное время
func (tr TimeRange) Start() time.Time {
	return tr.start
}

// End возвращает конечное время
func (tr TimeRange) End() time.Time {
	return tr.end
}

// Duration возвращает длительность диапазона
func (tr TimeRange) Duration() time.Duration {
	return tr.end.Sub(tr.start)
}

// Contains проверяет, попадает ли указанное время в диапазон
func (tr TimeRange) Contains(t time.Time) bool {
	return !t.Before(tr.start) && !t.After(tr.end)
}

// Overlaps проверяет, пересекаются ли два временных диапазона
func (tr TimeRange) Overlaps(other TimeRange) bool {
	return tr.start.Before(other.end) && other.start.Before(tr.end)
}
