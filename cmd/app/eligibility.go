package main

import "context"

type eligibilityChecker interface {
	CanRun(ctx context.Context) (bool, error)
}

type legacyReadinessChecker interface {
	LegacyTenantReady(ctx context.Context) (bool, error)
}

type combinedRunEligibility struct {
	setup  eligibilityChecker
	legacy legacyReadinessChecker
}

func (e combinedRunEligibility) CanRun(ctx context.Context) (bool, error) {
	setupComplete, err := e.setup.CanRun(ctx)
	if err != nil || !setupComplete {
		return setupComplete, err
	}
	return e.legacy.LegacyTenantReady(ctx)
}
