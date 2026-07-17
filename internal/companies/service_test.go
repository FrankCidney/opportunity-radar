package companies

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestFindOrCreateUsesStrongestAvailableIdentity(t *testing.T) {
	t.Parallel()

	source := "remotive"
	externalID := "company-42"
	existing := Company{
		ID:         42,
		Name:       "existing company",
		Source:     source,
		ExternalID: externalID,
		Domain:     "existing.example",
	}
	repo := &stubRepository{
		listResults: [][]Company{{existing}},
	}
	service := NewService(repo, discardLogger())

	got, err := service.FindOrCreate(context.Background(), &Company{
		Name:       "same name",
		Source:     source,
		ExternalID: externalID,
		Domain:     "different.example",
	})
	if err != nil {
		t.Fatalf("FindOrCreate() error = %v", err)
	}
	if got.ID != existing.ID {
		t.Fatalf("FindOrCreate() ID = %d, want %d", got.ID, existing.ID)
	}
	if len(repo.listFilters) != 1 {
		t.Fatalf("List() calls = %d, want 1", len(repo.listFilters))
	}

	filter := repo.listFilters[0]
	if filter.Source == nil || *filter.Source != source {
		t.Fatalf("source filter = %v, want %q", filter.Source, source)
	}
	if filter.ExternalID == nil || *filter.ExternalID != externalID {
		t.Fatalf("external ID filter = %v, want %q", filter.ExternalID, externalID)
	}
	if filter.Domain != nil || filter.Name != nil {
		t.Fatalf("strong identity lookup unexpectedly included weaker fields: %+v", filter)
	}
	if repo.created != nil {
		t.Fatal("Create() was called for an existing company")
	}
}

func TestFindOrCreateFallsBackFromExternalIdentityToDomainThenName(t *testing.T) {
	t.Parallel()

	existing := Company{ID: 7, Name: "acme", Domain: "acme.example"}
	repo := &stubRepository{
		listResults: [][]Company{
			nil,
			{existing},
		},
	}
	service := NewService(repo, discardLogger())

	got, err := service.FindOrCreate(context.Background(), &Company{
		Name:       "acme",
		Source:     "remotive",
		ExternalID: "missing",
		Domain:     "acme.example",
	})
	if err != nil {
		t.Fatalf("FindOrCreate() error = %v", err)
	}
	if got.ID != existing.ID {
		t.Fatalf("FindOrCreate() ID = %d, want %d", got.ID, existing.ID)
	}
	if len(repo.listFilters) != 2 {
		t.Fatalf("List() calls = %d, want 2", len(repo.listFilters))
	}
	if repo.listFilters[1].Domain == nil || *repo.listFilters[1].Domain != "acme.example" {
		t.Fatalf("second lookup = %+v, want domain lookup", repo.listFilters[1])
	}
	if repo.listFilters[1].Name != nil {
		t.Fatalf("domain match should stop before name lookup: %+v", repo.listFilters[1])
	}
}

func TestFindOrCreateCreatesWhenNoIdentityMatches(t *testing.T) {
	t.Parallel()

	repo := &stubRepository{
		listResults: [][]Company{nil, nil, nil},
		createID:    99,
	}
	service := NewService(repo, discardLogger())
	input := &Company{
		Name:       "new company",
		LogoURL:    "https://cdn.example/logo.png",
		Source:     "remotive",
		ExternalID: "new-99",
		Domain:     "new.example",
	}

	got, err := service.FindOrCreate(context.Background(), input)
	if err != nil {
		t.Fatalf("FindOrCreate() error = %v", err)
	}
	if got.ID != 99 {
		t.Fatalf("FindOrCreate() ID = %d, want 99", got.ID)
	}
	if repo.created == nil {
		t.Fatal("Create() was not called")
	}
	if repo.created == input {
		t.Fatal("FindOrCreate() should create a service-owned copy, not reuse input")
	}
	if repo.created.Name != input.Name ||
		repo.created.Source != input.Source ||
		repo.created.ExternalID != input.ExternalID ||
		repo.created.Domain != input.Domain {
		t.Fatalf("created company = %+v, want input identity %+v", repo.created, input)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type stubRepository struct {
	listResults [][]Company
	listFilters []CompanyListFilter
	listCalls   int
	created     *Company
	createID    int64
	createErr   error
}

func (r *stubRepository) Create(_ context.Context, company *Company) error {
	r.created = company
	if r.createErr != nil {
		return r.createErr
	}
	company.ID = r.createID
	return nil
}

func (r *stubRepository) GetByID(context.Context, int64) (*Company, error) {
	panic("unexpected GetByID call")
}

func (r *stubRepository) Update(context.Context, *Company) error {
	panic("unexpected Update call")
}

func (r *stubRepository) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (r *stubRepository) List(_ context.Context, filter CompanyListFilter) ([]Company, error) {
	r.listFilters = append(r.listFilters, filter)
	call := r.listCalls
	r.listCalls++
	if call >= len(r.listResults) {
		return nil, nil
	}
	return r.listResults[call], nil
}
