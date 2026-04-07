package main

import (
	"testing"

	"opportunity-radar/internal/preferences"
)

func TestDeriveBrighterMondayListingPathsDefaultsToSoftwareData(t *testing.T) {
	t.Parallel()

	got := deriveBrighterMondayListingPaths(nil)
	want := []string{"/jobs/software-data"}

	if !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected paths: got %v want %v", got, want)
	}
}

func TestDeriveBrighterMondayListingPathsAddsRemotePathWhenPreferred(t *testing.T) {
	t.Parallel()

	settings := &preferences.Settings{
		DesiredRoles: []string{"Backend Engineer"},
		WorkModes:    []string{"Remote"},
		Locations:    []string{"Kenya"},
	}

	got := deriveBrighterMondayListingPaths(settings)
	want := []string{"/jobs/software-data", "/jobs/software-data/remote"}

	if !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected paths: got %v want %v", got, want)
	}
}

func stringSlicesEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
