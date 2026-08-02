// Package policyv1 defines deterministic authorization policy and billing-period behavior.
package policyv1

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"
)

const (
	ResetNever        = "never"
	ResetDaily        = "daily"
	ResetWeekly       = "weekly"
	ResetMonthly      = "monthly"
	ResetIntervalDays = "interval_days"
)

var (
	identifierPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	periodIDPattern   = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

type ResetRule struct {
	Kind         string     `json:"kind"`
	Value        *uint32    `json:"value,omitempty"`
	Timezone     string     `json:"timezone"`
	PeriodAnchor *time.Time `json:"period_anchor,omitempty"`
}

type Period struct {
	ID       string     `json:"id"`
	StartsAt time.Time  `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
}

func ValidateIdentifier(field, value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("%s: invalid identifier", field)
	}
	return nil
}

func ValidateResetRule(value ResetRule) error {
	if value.Timezone == "" || value.Timezone == "Local" {
		return errors.New("timezone: must be UTC or an IANA timezone")
	}
	if _, err := time.LoadLocation(value.Timezone); err != nil {
		return errors.New("timezone: must be UTC or an IANA timezone")
	}
	switch value.Kind {
	case ResetNever, ResetDaily:
		if value.Value != nil || value.PeriodAnchor != nil {
			return fmt.Errorf("%s reset must not include value or period_anchor", value.Kind)
		}
	case ResetWeekly:
		if value.Value == nil || *value.Value < 1 || *value.Value > 7 || value.PeriodAnchor != nil {
			return errors.New("weekly reset value must be an ISO weekday from 1 to 7")
		}
	case ResetMonthly:
		if value.Value == nil || *value.Value < 1 || *value.Value > 31 || value.PeriodAnchor != nil {
			return errors.New("monthly reset value must be a day from 1 to 31")
		}
	case ResetIntervalDays:
		if value.Value == nil || *value.Value < 1 || *value.Value > 3650 || value.PeriodAnchor == nil || value.PeriodAnchor.IsZero() {
			return errors.New("interval_days reset requires a 1 to 3650 day value and period_anchor")
		}
	default:
		return fmt.Errorf("kind: unsupported reset kind %q", value.Kind)
	}
	return nil
}

// CurrentPeriod returns the same period on the center and Agent for a given instant.
// authorizationStartedAt is the beginning of a never-reset period and prevents any
// recurring period from predating the authorization.
func CurrentPeriod(value ResetRule, authorizationStartedAt, at time.Time) (Period, error) {
	if err := ValidateResetRule(value); err != nil {
		return Period{}, err
	}
	if authorizationStartedAt.IsZero() || at.IsZero() {
		return Period{}, errors.New("authorization start and evaluation time must be set")
	}
	authorizationStartedAt = authorizationStartedAt.UTC()
	at = at.UTC()
	if at.Before(authorizationStartedAt) {
		return Period{}, errors.New("evaluation time precedes authorization start")
	}
	location, _ := time.LoadLocation(value.Timezone)
	var start, end time.Time
	switch value.Kind {
	case ResetNever:
		start = authorizationStartedAt
	case ResetDaily:
		local := at.In(location)
		start = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
		end = start.AddDate(0, 0, 1)
	case ResetWeekly:
		local := at.In(location)
		isoWeekday := int(local.Weekday())
		if isoWeekday == 0 {
			isoWeekday = 7
		}
		daysBack := (isoWeekday - int(*value.Value) + 7) % 7
		start = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -daysBack)
		end = start.AddDate(0, 0, 7)
	case ResetMonthly:
		local := at.In(location)
		year, month := local.Year(), local.Month()
		start = monthlyBoundary(year, month, int(*value.Value), location)
		if start.After(local) {
			year, month = adjacentMonth(year, month, -1)
			start = monthlyBoundary(year, month, int(*value.Value), location)
		}
		year, month = adjacentMonth(year, month, 1)
		end = monthlyBoundary(year, month, int(*value.Value), location)
	case ResetIntervalDays:
		anchor := value.PeriodAnchor.In(location)
		local := at.In(location)
		elapsedDays := civilDay(local) - civilDay(anchor)
		step := int64(*value.Value)
		periodIndex := elapsedDays / step
		if elapsedDays < 0 && elapsedDays%step != 0 {
			periodIndex--
		}
		start = anchor.AddDate(0, 0, int(periodIndex*step))
		for start.After(local) {
			start = start.AddDate(0, 0, -int(step))
		}
		end = start.AddDate(0, 0, int(step))
	}
	if start.Before(authorizationStartedAt) {
		start = authorizationStartedAt
	}
	start = start.UTC()
	period := Period{StartsAt: start}
	if !end.IsZero() {
		end = end.UTC()
		period.EndsAt = &end
	}
	period.ID = periodID(period.StartsAt, period.EndsAt)
	return period, nil
}

func ValidatePeriod(value Period) error {
	if !periodIDPattern.MatchString(value.ID) {
		return errors.New("period.id: invalid period ID")
	}
	if value.StartsAt.IsZero() {
		return errors.New("period.starts_at: required")
	}
	if value.EndsAt != nil && (value.EndsAt.IsZero() || !value.EndsAt.After(value.StartsAt)) {
		return errors.New("period.ends_at: must be after starts_at")
	}
	if value.ID != periodID(value.StartsAt.UTC(), value.EndsAt) {
		return errors.New("period.id: does not match boundaries")
	}
	return nil
}

func SamePeriod(first, second Period) bool {
	if first.ID != second.ID || !first.StartsAt.Equal(second.StartsAt) {
		return false
	}
	if first.EndsAt == nil || second.EndsAt == nil {
		return first.EndsAt == nil && second.EndsAt == nil
	}
	return first.EndsAt.Equal(*second.EndsAt)
}

func monthlyBoundary(year int, month time.Month, day int, location *time.Location) time.Time {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, location).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, 0, 0, 0, 0, location)
}

func adjacentMonth(year int, month time.Month, delta int) (int, time.Month) {
	index := year*12 + int(month) - 1 + delta
	return index / 12, time.Month(index%12 + 1)
}

func civilDay(value time.Time) int64 {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC).Unix() / 86400
}

func periodID(start time.Time, end *time.Time) string {
	input := start.UTC().Format(time.RFC3339Nano) + "\x00"
	if end != nil {
		input += end.UTC().Format(time.RFC3339Nano)
	}
	digest := sha256.Sum256([]byte(input))
	return hex.EncodeToString(digest[:16])
}
