package server

import "github.com/stockyard-dev/stockyard-brand-standalone/internal/license"

// Limits holds the feature limits for the current license tier.
// All int limits: 0 means unlimited (Pro tier only).
type Limits struct {
	MaxEventsPerMonth int // 0 = unlimited (Pro)
	RetentionDays int // 0 = unlimited (Pro)
	MaxWorkspaces int // 0 = unlimited (Pro)
	PolicyTemplates bool
	ScheduledExport bool
	SignedBundles bool
	ChainHealthCheck bool
}

var freeLimits = Limits{
		MaxEventsPerMonth: 10000,
		RetentionDays: 7,
		MaxWorkspaces: 1,
		PolicyTemplates: false,
		ScheduledExport: false,
		SignedBundles: false,
		ChainHealthCheck: false,
}

var proLimits = Limits{
		MaxEventsPerMonth: 0,
		RetentionDays: 90,
		MaxWorkspaces: 0,
		PolicyTemplates: true,
		ScheduledExport: true,
		SignedBundles: true,
		ChainHealthCheck: true,
}

// LimitsFor returns the appropriate Limits for the given license info.
// nil info = no key set = free tier.
func LimitsFor(info *license.Info) Limits {
	if info != nil && info.IsPro() {
		return proLimits
	}
	return freeLimits
}

// LimitReached returns true if the current count meets or exceeds the limit.
// A limit of 0 is treated as unlimited.
func LimitReached(limit, current int) bool {
	if limit == 0 {
		return false
	}
	return current >= limit
}
