package main

import (
	"strings"

	"opportunity-radar/internal/preferences"
	"opportunity-radar/internal/scrapers/brightermonday"
)

func buildBrighterMondayConfig(settings *preferences.Settings) brightermonday.Config {
	paths := deriveBrighterMondayListingPaths(settings)
	return brightermonday.Config{
		ListingPaths:    paths,
		MaxPagesPerPath: 1,
	}
}

func deriveBrighterMondayListingPaths(settings *preferences.Settings) []string {
	const (
		softwareDataPath = "/jobs/software-data"
		remotePath       = "/jobs/software-data/remote"
	)

	paths := []string{softwareDataPath}
	if settings == nil {
		return paths
	}

	if prefersRemote(settings.WorkModes, settings.Locations) {
		paths = append(paths, remotePath)
	}

	return dedupeStrings(paths)
}

func prefersRemote(workModes []string, locations []string) bool {
	for _, value := range append(append([]string{}, workModes...), locations...) {
		if strings.EqualFold(strings.TrimSpace(value), "remote") {
			return true
		}
	}
	return false
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}
