package brightermonday

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestParseListingPageExtractsCardsAndNextPage(t *testing.T) {
	t.Parallel()

	doc := mustParseHTML(t, `
	<html><body>
	  <section>
	    <article class="job-card">
	      <a href="/listings/backend-developer-abc123">Backend Developer</a>
	      <p>Acme Labs</p>
	      <p>Remote (Work From Home) Full Time KSh 90,000 - 105,000</p>
	      <p>Software & Data</p>
	      <p>4 weeks ago</p>
	      <img src="/logos/acme.png" />
	    </article>
	    <article class="job-card">
	      <a href="/listings/data-engineer-def456">Data Engineer</a>
	      <p>Orbit Analytics</p>
	      <p>Nairobi Full Time</p>
	      <p>Software & Data</p>
	      <p>1 week ago</p>
	    </article>
	    <nav>
	      <a href="/jobs/software-data?page=2" aria-label="Next page">Next</a>
	    </nav>
	  </section>
	</body></html>`)

	base := mustParseURL(t, "https://www.brightermonday.co.ke")
	candidates, nextURL := parseListingPage(doc, base, "https://www.brightermonday.co.ke/jobs/software-data")

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	first := candidates[0]
	if first.Title != "Backend Developer" {
		t.Fatalf("unexpected first title: %q", first.Title)
	}
	if first.Company != "Acme Labs" {
		t.Fatalf("unexpected first company: %q", first.Company)
	}
	if first.Location != "Remote (Work From Home)" {
		t.Fatalf("unexpected first location: %q", first.Location)
	}
	if first.WorkType != "Full Time" {
		t.Fatalf("unexpected first work type: %q", first.WorkType)
	}
	if first.Salary != "KSh 90,000 - 105,000" {
		t.Fatalf("unexpected first salary: %q", first.Salary)
	}
	if first.RelativeAge != "4 weeks ago" {
		t.Fatalf("unexpected first relative age: %q", first.RelativeAge)
	}
	if first.CompanyLogo != "https://www.brightermonday.co.ke/logos/acme.png" {
		t.Fatalf("unexpected first logo: %q", first.CompanyLogo)
	}

	if nextURL != "https://www.brightermonday.co.ke/jobs/software-data?page=2" {
		t.Fatalf("unexpected next URL: %q", nextURL)
	}
}

func TestParseDetailPageExtractsStructuredFields(t *testing.T) {
	t.Parallel()

	doc := mustParseHTML(t, `
	<html><body>
	  <main>
	    <p>1 month ago</p>
	    <h1>TECHNICAL ASSOCIATE</h1>
	    <h2>ACF Gas Ltd</h2>
	    <p>Software & Data</p>
	    <p>Mombasa</p>
	    <p>Full Time</p>
	    <p>KSh 45,000 - 60,000</p>
	    <img src="/logos/acf.png" />
	    <h3>Job Summary</h3>
	    <p>We are looking for a hands-on ERPNext Technical Associate.</p>
	    <p>Minimum Qualification : Bachelors</p>
	    <p>Experience Level : Entry level</p>
	    <p>Experience Length : 1 year</p>
	    <p>Working Hours : Full Time</p>
	    <h3>Job Description/Requirements</h3>
	    <p>Assist with configuration and system testing.</p>
	    <p>Document workflows and integrations.</p>
	    <h3>Important Safety Tips</h3>
	  </main>
	</body></html>`)

	base := mustParseURL(t, "https://www.brightermonday.co.ke")
	before := time.Now().UTC().AddDate(0, -1, -2)
	after := time.Now().UTC().AddDate(0, -1, 2)

	detail, err := parseDetailPage(doc, base, "https://www.brightermonday.co.ke/listings/technical-associate-7w29kn")
	if err != nil {
		t.Fatalf("expected detail parse to succeed: %v", err)
	}

	if detail.Title != "TECHNICAL ASSOCIATE" {
		t.Fatalf("unexpected title: %q", detail.Title)
	}
	if detail.Company != "ACF Gas Ltd" {
		t.Fatalf("unexpected company: %q", detail.Company)
	}
	if detail.Location != "Mombasa" {
		t.Fatalf("unexpected location: %q", detail.Location)
	}
	if detail.WorkType != "Full Time" {
		t.Fatalf("unexpected work type: %q", detail.WorkType)
	}
	if detail.Salary != "KSh 45,000 - 60,000" {
		t.Fatalf("unexpected salary: %q", detail.Salary)
	}
	if detail.ExperienceLevel != "Entry level" {
		t.Fatalf("unexpected experience level: %q", detail.ExperienceLevel)
	}
	if detail.ExperienceLength != "1 year" {
		t.Fatalf("unexpected experience length: %q", detail.ExperienceLength)
	}
	if detail.Qualification != "Bachelors" {
		t.Fatalf("unexpected qualification: %q", detail.Qualification)
	}
	if detail.CompanyLogo != "https://www.brightermonday.co.ke/logos/acf.png" {
		t.Fatalf("unexpected logo: %q", detail.CompanyLogo)
	}
	if !strings.Contains(detail.Description, "Assist with configuration and system testing.") {
		t.Fatalf("unexpected description: %q", detail.Description)
	}
	if detail.PostedAt.Before(before) || detail.PostedAt.After(after) {
		t.Fatalf("unexpected posted time: %s", detail.PostedAt)
	}
}

func TestParseDetailPageHandlesLiveHeadingVariants(t *testing.T) {
	t.Parallel()

	doc := mustParseHTML(t, `
	<html><body>
	  <main>
	    <h1>UI/UX Designer</h1>
	    <h2>Trackalways Africa Limited</h2>
	    <p>1 week ago</p>
	    <p>Nairobi</p>
	    <p>Full Time</p>
	    <p>KSh 30,000 - 45,000</p>
	    <h3>Job summary</h3>
	    <p>A UI/UX Designer is responsible for designing digital products.</p>
	    <p>Min Qualification: Bachelors Experience Level: Executive level Experience Length: 2 years Working Hours: Full Time</p>
	    <h3>Job descriptions & requirements</h3>
	    <p>* Conduct user research to understand user needs.</p>
	    <p>* Create wireframes, mockups, and prototypes.</p>
	    <h3>Important safety tips</h3>
	  </main>
	</body></html>`)

	base := mustParseURL(t, "https://www.brightermonday.co.ke")
	detail, err := parseDetailPage(doc, base, "https://www.brightermonday.co.ke/listings/ui-ux-designer-xp4n24")
	if err != nil {
		t.Fatalf("expected detail parse to succeed: %v", err)
	}

	if !strings.Contains(detail.Description, "A UI/UX Designer is responsible for designing digital products.") {
		t.Fatalf("expected summary in description, got %q", detail.Description)
	}
	if !strings.Contains(detail.Description, "* Conduct user research to understand user needs.") {
		t.Fatalf("expected requirements in description, got %q", detail.Description)
	}
	if detail.Qualification != "Bachelors" {
		t.Fatalf("unexpected qualification: %q", detail.Qualification)
	}
	if detail.ExperienceLevel != "Executive level" {
		t.Fatalf("unexpected experience level: %q", detail.ExperienceLevel)
	}
	if detail.ExperienceLength != "2 years" {
		t.Fatalf("unexpected experience length: %q", detail.ExperienceLength)
	}
}

func TestScrapeFollowsPaginationAndFetchesDetailPages(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/software-data", func(w http.ResponseWriter, r *http.Request) {
		switch got := r.URL.Query().Get("page"); got {
		case "":
			_, _ = io.WriteString(w, `
		<html><body>
		  <article>
		    <a href="/listings/backend-developer-abc123">Backend Developer</a>
		    <p>Acme Labs</p>
		    <p>Remote (Work From Home) Full Time KSh 90,000 - 105,000</p>
		    <p>Software & Data</p>
		    <p>4 weeks ago</p>
		  </article>
		  <a href="/jobs/software-data?page=2" rel="next">Next</a>
		</body></html>`)
		case "2":
			_, _ = io.WriteString(w, `
		<html><body>
		  <article>
		    <a href="/listings/data-engineer-def456">Data Engineer</a>
		    <p>Orbit Analytics</p>
		    <p>Nairobi Full Time</p>
		    <p>Software & Data</p>
		    <p>1 week ago</p>
		  </article>
		</body></html>`)
		default:
			t.Fatalf("unexpected page query: %q", got)
		}
	})
	mux.HandleFunc("/listings/backend-developer-abc123", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `
		<html><body>
		  <h1>Backend Developer</h1>
		  <h2>Acme Labs</h2>
		  <p>4 weeks ago</p>
		  <p>Remote (Work From Home)</p>
		  <p>Full Time</p>
		  <p>KSh 90,000 - 105,000</p>
		  <h3>Job Summary</h3>
		  <p>Build backend services.</p>
		  <h3>Job Description/Requirements</h3>
		  <p>Work with NestJS and PostgreSQL.</p>
		  <p>Experience Level : Mid level</p>
		  <h3>Important Safety Tips</h3>
		</body></html>`)
	})
	mux.HandleFunc("/listings/data-engineer-def456", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `
		<html><body>
		  <h1>Data Engineer</h1>
		  <h2>Orbit Analytics</h2>
		  <p>1 week ago</p>
		  <p>Nairobi</p>
		  <p>Full Time</p>
		  <h3>Job Summary</h3>
		  <p>Build pipelines.</p>
		  <h3>Job Description/Requirements</h3>
		  <p>Maintain ETL workflows.</p>
		  <h3>Important Safety Tips</h3>
		</body></html>`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	scraper := newScraper(Config{
		BaseURL:         server.URL,
		ListingPaths:    []string{"/jobs/software-data"},
		MaxPagesPerPath: 2,
	}, server.Client(), testLogger())

	raws, err := scraper.Scrape(context.Background())
	if err != nil {
		t.Fatalf("expected scrape to succeed: %v", err)
	}

	if len(raws) != 2 {
		t.Fatalf("expected 2 raw jobs, got %d", len(raws))
	}

	if raws[0].Source != "brightermonday" {
		t.Fatalf("unexpected source: %q", raws[0].Source)
	}
	if raws[0].ExternalID == "" {
		t.Fatalf("expected external id to be derived")
	}
}

func TestParseRelativePostedAt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		input string
		want  time.Time
	}{
		{input: "New", want: now},
		{input: "1 week ago", want: now.AddDate(0, 0, -7)},
		{input: "2 weeks ago", want: now.AddDate(0, 0, -14)},
		{input: "1 month ago", want: now.AddDate(0, -1, 0)},
	}

	for _, tc := range cases {
		got, ok := parseRelativePostedAt(tc.input, now)
		if !ok {
			t.Fatalf("expected %q to parse", tc.input)
		}
		if !got.Equal(tc.want) {
			t.Fatalf("unexpected parsed time for %q: got %s want %s", tc.input, got, tc.want)
		}
	}
}

func TestNewScraperDefaults(t *testing.T) {
	t.Parallel()

	scraper := newScraper(Config{}, nil, testLogger())

	if scraper.maxPages != defaultMaxPages {
		t.Fatalf("unexpected max pages: got %d want %d", scraper.maxPages, defaultMaxPages)
	}
	if len(scraper.listingPaths) != 1 || scraper.listingPaths[0] != "/jobs/software-data" {
		t.Fatalf("unexpected listing paths: %v", scraper.listingPaths)
	}
	if scraper.client == nil {
		t.Fatal("expected default client")
	}
}

func mustParseHTML(t *testing.T, raw string) *html.Node {
	t.Helper()

	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("failed to parse HTML: %v", err)
	}
	return doc
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}
	return parsed
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
