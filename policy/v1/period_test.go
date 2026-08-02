package policyv1

import (
	"testing"
	"time"
)

func uint32Pointer(value uint32) *uint32 { return &value }

func TestCurrentPeriodResetKinds(t *testing.T) {
	started := time.Date(2026, time.January, 20, 15, 30, 0, 0, time.UTC)
	tests := []struct {
		name      string
		rule      ResetRule
		at        time.Time
		wantStart string
		wantEnd   string
	}{
		{name: "never", rule: ResetRule{Kind: ResetNever, Timezone: "UTC"}, at: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC), wantStart: "2026-01-20T15:30:00Z"},
		{name: "daily DST", rule: ResetRule{Kind: ResetDaily, Timezone: "America/New_York"}, at: time.Date(2026, 3, 8, 18, 0, 0, 0, time.UTC), wantStart: "2026-03-08T05:00:00Z", wantEnd: "2026-03-09T04:00:00Z"},
		{name: "weekly Sunday", rule: ResetRule{Kind: ResetWeekly, Value: uint32Pointer(7), Timezone: "Asia/Shanghai"}, at: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC), wantStart: "2026-08-01T16:00:00Z", wantEnd: "2026-08-08T16:00:00Z"},
		{name: "monthly clamps", rule: ResetRule{Kind: ResetMonthly, Value: uint32Pointer(31), Timezone: "UTC"}, at: time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC), wantStart: "2026-04-30T00:00:00Z", wantEnd: "2026-05-31T00:00:00Z"},
		{name: "monthly January does not skip February", rule: ResetRule{Kind: ResetMonthly, Value: uint32Pointer(31), Timezone: "UTC"}, at: time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC), wantStart: "2026-01-31T00:00:00Z", wantEnd: "2026-02-28T00:00:00Z"},
		{name: "interval preserves local time", rule: ResetRule{Kind: ResetIntervalDays, Value: uint32Pointer(7), Timezone: "America/New_York", PeriodAnchor: timePointer(time.Date(2026, 3, 1, 15, 0, 0, 0, time.UTC))}, at: time.Date(2026, 3, 15, 16, 0, 0, 0, time.UTC), wantStart: "2026-03-15T14:00:00Z", wantEnd: "2026-03-22T14:00:00Z"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			period, err := CurrentPeriod(test.rule, started, test.at)
			if err != nil {
				t.Fatalf("CurrentPeriod() error = %v", err)
			}
			if got := period.StartsAt.Format(time.RFC3339); got != test.wantStart {
				t.Fatalf("start = %s, want %s", got, test.wantStart)
			}
			if test.wantEnd == "" {
				if period.EndsAt != nil {
					t.Fatalf("end = %s, want nil", period.EndsAt)
				}
			} else if period.EndsAt == nil || period.EndsAt.Format(time.RFC3339) != test.wantEnd {
				t.Fatalf("end = %v, want %s", period.EndsAt, test.wantEnd)
			}
			if err := ValidatePeriod(period); err != nil {
				t.Fatalf("ValidatePeriod() error = %v", err)
			}
		})
	}
}

func TestCurrentPeriodDoesNotPredateAuthorization(t *testing.T) {
	started := time.Date(2026, 8, 2, 12, 34, 0, 0, time.UTC)
	period, err := CurrentPeriod(ResetRule{Kind: ResetDaily, Timezone: "UTC"}, started, started.Add(time.Minute))
	if err != nil || !period.StartsAt.Equal(started) {
		t.Fatalf("CurrentPeriod() = %+v, %v", period, err)
	}
}

func timePointer(value time.Time) *time.Time { return &value }
