package fuzu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/net/html"

	"opportunity-radar/internal/ingest/normalize"
)

const (
	sourceID            = "fuzu"
	defaultBaseURL      = "https://www.fuzu.com"
	defaultListingPath  = "/kenya/job/computers-software-development"
	defaultMaxPages     = 3
	defaultHTTPTimeout  = 15 * time.Second
	defaultRequestDelay = 1 * time.Second
	defaultUserAgent    = "opportunity-radar/1.0 (+https://github.com/FrankCidney/opportunity-radar.git)"
)

type Config struct {
	BaseURL         string
	ListingPaths    []string
	MaxPagesPerPath int
	RequestDelay    time.Duration
}

type Scraper struct {
	baseURL       *url.URL
	listingPaths  []string
	maxPages       int
	requestDelay  time.Duration
	client        *http.Client
	logger        *slog.Logger
	requestMu     sync.Mutex
	lastRequestAt time.Time
}

type listingCandidate struct {
	Title   string
	Company string
	URL     string
}

type detailJob struct {
	Title       string
	Company    string
	Description string
	Location   string
	JobType    string
	Salary     string
	PostedAt   time.Time
	ExternalID string
	RawData    map[string]interface{}
}

type ItemListElement struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type JobPostingData struct {
	ExternalID     string
	DatePosted     string
	Description    string
	Title          string
	URL            string
	EmploymentType string
	ValidThrough   string
	Industry       string
	Skills         string
	Company        string
	Location       string
	Salary         string
}

var (
	reWhitespace       = regexp.MustCompile(`\s+`)
	reRelativeDate     = regexp.MustCompile(`(?i)^(new|just posted|\d+\s+(minute|minutes|hour|hours|day|days|week|weeks|month|months)\s+ago)$`)
	rePaginationNumber = regexp.MustCompile(`^\d+$`)
)

func NewScraper(cfg Config, logger *slog.Logger) *Scraper {
	if cfg.RequestDelay == 0 {
		cfg.RequestDelay = defaultRequestDelay
	}
	return newScraper(cfg, &http.Client{Timeout: defaultHTTPTimeout}, logger)
}

func newScraper(cfg Config, client *http.Client, logger *slog.Logger) *Scraper {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		parsedBase, _ = url.Parse(defaultBaseURL)
	}

	listingPaths := normalizeListingPaths(cfg.ListingPaths)
	if len(listingPaths) == 0 {
		listingPaths = []string{defaultListingPath}
	}

	maxPages := cfg.MaxPagesPerPath
	if maxPages <= 0 {
		maxPages = defaultMaxPages
	}

	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}

	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Scraper{
		baseURL:      parsedBase,
		listingPaths: listingPaths,
		maxPages:     maxPages,
		requestDelay: cfg.RequestDelay,
		client:       client,
		logger:       logger,
	}
}

func (s *Scraper) Source() string {
	return sourceID
}

func (s *Scraper) Scrape(ctx context.Context) ([]normalize.RawJob, error) {
	discovered := make(map[string]listingCandidate)

	for _, listingPath := range s.listingPaths {
		if err := s.scrapeListingPath(ctx, listingPath, discovered); err != nil {
			return nil, err
		}
	}

	urls := make([]string, 0, len(discovered))
	for rawURL := range discovered {
		urls = append(urls, rawURL)
	}
	sort.Strings(urls)

	raws := make([]normalize.RawJob, 0, len(urls))
	for _, rawURL := range urls {
		candidate := discovered[rawURL]
		detail, err := s.fetchDetail(ctx, candidate.URL)
		if err != nil {
			s.logger.Warn("failed to fetch Fuzu detail page", "url", candidate.URL, "error", err)
			continue
		}

		raws = append(raws, s.toRawJob(candidate, detail))
	}

	s.logger.Info("fuzu scrape complete", "count", len(raws))
	return raws, nil
}

func (s *Scraper) scrapeListingPath(ctx context.Context, listingPath string, discovered map[string]listingCandidate) error {
	nextURL := s.resolveURL(listingPath)

	for page := 1; page <= s.maxPages && nextURL != ""; page++ {
		doc, err := s.fetchHTML(ctx, nextURL)
		if err != nil {
			return fmt.Errorf("fetch listing page %q: %w", nextURL, err)
		}

		candidates, parsedNextURL := parseListingPage(doc, s.baseURL, nextURL)
		for _, candidate := range candidates {
			if candidate.URL == "" {
				continue
			}
			if _, exists := discovered[candidate.URL]; exists {
				continue
			}
			discovered[candidate.URL] = candidate
		}

		if parsedNextURL == nextURL || parsedNextURL == "" {
			break
		}
		nextURL = parsedNextURL
	}

	return nil
}

func (s *Scraper) fetchDetail(ctx context.Context, rawURL string) (detailJob, error) {
	doc, err := s.fetchHTML(ctx, rawURL)
	if err != nil {
		return detailJob{}, err
	}

	detail, err := parseDetailPage(doc, s.baseURL, rawURL)
	if err != nil {
		return detailJob{}, err
	}

	return detail, nil
}

func (s *Scraper) fetchHTML(ctx context.Context, rawURL string) (*html.Node, error) {
	if err := s.waitForRequestSlot(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	return doc, nil
}

func (s *Scraper) waitForRequestSlot(ctx context.Context) error {
	if s.requestDelay <= 0 {
		return nil
	}

	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	if !s.lastRequestAt.IsZero() {
		wait := time.Until(s.lastRequestAt.Add(s.requestDelay))
		if wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	s.lastRequestAt = time.Now()
	return nil
}

func (s *Scraper) toRawJob(candidate listingCandidate, detail detailJob) normalize.RawJob {
	title := firstNonEmpty(detail.Title, candidate.Title)
	company := firstNonEmpty(detail.Company, candidate.Company)
	location := detail.Location
	jobType := detail.JobType
	salary := detail.Salary
	postedAt := detail.PostedAt.UTC().Format(time.RFC3339)
	externalID := firstNonEmpty(detail.ExternalID, externalIDFromURL(candidate.URL))

	rawData := detail.RawData
	if rawData == nil {
		rawData = map[string]interface{}{}
	}
	rawData["source_page"] = candidate.URL

	return normalize.RawJob{
		Source:      s.Source(),
		Title:       title,
		Company:     company,
		ExternalID:  externalID,
		Location:    location,
		JobType:     jobType,
		Salary:      salary,
		Description: detail.Description,
		URL:         candidate.URL,
		PostedAt:    postedAt,
		RawData:     rawData,
	}
}

func parseListingPage(doc *html.Node, baseURL *url.URL, pageURL string) ([]listingCandidate, string) {
	itemList := extractJSONLDItemList(doc)
	candidates := make([]listingCandidate, 0, len(itemList))

	for _, item := range itemList {
		resolvedURL := resolveURL(baseURL, item.URL)
		if resolvedURL == "" {
			continue
		}

		candidates = append(candidates, listingCandidate{
			Title: strings.TrimSpace(item.Name),
			URL:   resolvedURL,
		})
	}

	if len(candidates) == 0 {
		candidates = parseListingPageHTML(doc, baseURL)
	}

	nextURL := parseNextPageURL(doc, baseURL, pageURL)
	return candidates, nextURL
}

func extractJSONLDItemList(doc *html.Node) []ItemListElement {
	var items []ItemListElement
	scripts := extractJSONLDScripts(doc, "ItemList")

	for _, raw := range scripts {
		var itemList struct {
			ItemListElement []ItemListElement `json:"itemListElement"`
		}
		if err := json.Unmarshal([]byte(raw), &itemList); err != nil {
			continue
		}
		if len(itemList.ItemListElement) > 0 {
			items = itemList.ItemListElement
			break
		}
	}

	return items
}

func extractJSONLDJobPosting(doc *html.Node) JobPostingData {
	scripts := extractJSONLDScripts(doc, "JobPosting")

	for _, raw := range scripts {
		job, err := parseJobPosting(raw)
		if err != nil {
			continue
		}
		if job.Title != "" || job.Description != "" {
			return job
		}
	}

	return JobPostingData{}
}

func parseJobPosting(raw string) (JobPostingData, error) {
	var wrapper struct {
		Type             string `json:"@type"`
		Title            string `json:"title"`
		URL              string `json:"url"`
		DatePosted       string `json:"datePosted"`
		ValidThrough     string `json:"validThrough"`
		EmploymentType   string `json:"employmentType"`
		Industry         string `json:"industry"`
		Skills           string `json:"skills"`
		Description      string `json:"description"`
		Identifier       struct {
			Value string `json:"value"`
		} `json:"identifier"`
		HiringOrganization struct {
			Name string `json:"name"`
		} `json:"hiringOrganization"`
		JobLocation struct {
			Address struct {
				AddressLocality string `json:"addressLocality"`
			} `json:"address"`
		} `json:"jobLocation"`
	}

	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return JobPostingData{}, err
	}

	if wrapper.Type != "JobPosting" {
		return JobPostingData{}, fmt.Errorf("not a JobPosting")
	}

	description := wrapper.Description
	description = stripHTML(description)
	description = cleanDescriptionSections(description)

	return JobPostingData{
		ExternalID:     strings.TrimSpace(wrapper.Identifier.Value),
		DatePosted:     strings.TrimSpace(wrapper.DatePosted),
		Description:    description,
		Title:          strings.TrimSpace(wrapper.Title),
		URL:            strings.TrimSpace(wrapper.URL),
		EmploymentType: strings.TrimSpace(wrapper.EmploymentType),
		ValidThrough:   strings.TrimSpace(wrapper.ValidThrough),
		Industry:       strings.TrimSpace(wrapper.Industry),
		Skills:         strings.TrimSpace(wrapper.Skills),
		Company:        strings.TrimSpace(wrapper.HiringOrganization.Name),
		Location:       strings.TrimSpace(wrapper.JobLocation.Address.AddressLocality),
	}, nil
}

func stripHTML(value string) string {
	doc, err := html.Parse(strings.NewReader(value))
	if err != nil {
		return value
	}

	var b strings.Builder
	var visit func(*html.Node)

	visit = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode && (node.Data == "script" || node.Data == "style") {
			return
		}
		if node.Type == html.TextNode {
			text := cleanText(node.Data)
			if text != "" {
				b.WriteString(text)
				b.WriteByte('\n')
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}

	visit(doc)
	return strings.TrimSpace(b.String())
}

func cleanDescriptionSections(description string) string {
	if description == "" {
		return ""
	}

	lines := strings.Split(description, "\n")
	var cleaned []string

	for _, line := range lines {
		trimmed := cleanText(line)
		if trimmed == "" {
			continue
		}

		lower := strings.ToLower(trimmed)
		switch lower {
		case "requirements":
			continue
		case "qualifications":
			continue
		}

		cleaned = append(cleaned, trimmed)
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func parseDetailPage(doc *html.Node, baseURL *url.URL, pageURL string) (detailJob, error) {
	job := extractJSONLDJobPosting(doc)

	detail := detailJob{
		Title:       strings.TrimSpace(job.Title),
		Company:     strings.TrimSpace(job.Company),
		Description: strings.TrimSpace(job.Description),
		Location:    strings.TrimSpace(job.Location),
		JobType:     strings.TrimSpace(job.EmploymentType),
		Salary:      strings.TrimSpace(job.Salary),
		ExternalID:  job.ExternalID,
		RawData: map[string]interface{}{
			"industry":      strings.TrimSpace(job.Industry),
			"skills":        strings.TrimSpace(job.Skills),
			"valid_through": strings.TrimSpace(job.ValidThrough),
		},
	}

	if detail.Description == "" {
		desc := extractDetailDescriptionHTML(doc)
		detail.Description = strings.TrimSpace(desc)
	}

	if detail.Title == "" {
		detail.Title = firstHeadingText(doc, "h1")
	}

	if detail.Company == "" {
		detail.Company = firstHeadingText(doc, "h2")
	}

	if detail.Description == "" {
		return detailJob{}, fmt.Errorf("missing description")
	}

	if detail.PostedAt.IsZero() {
		if job.DatePosted != "" {
			parsed, err := time.Parse(time.RFC3339, job.DatePosted)
			if err != nil {
				parsed, err = time.Parse("2006-01-02", job.DatePosted)
				if err != nil {
					parsed = time.Time{}
				}
			}
			if !parsed.IsZero() {
				detail.PostedAt = parsed.UTC()
			}
		}
	}

	if detail.PostedAt.IsZero() {
		lines := textLines(doc)
		for _, line := range lines {
			if postedAt, ok := parseRelativePostedAt(line, time.Now().UTC()); ok {
				detail.PostedAt = postedAt
				break
			}
		}
	}

	if detail.PostedAt.IsZero() {
		detail.PostedAt = time.Now().UTC()
	}

	if detail.ExternalID == "" {
		detail.ExternalID = externalIDFromURL(pageURL)
	}

	return detail, nil
}

func parseListingPageHTML(doc *html.Node, baseURL *url.URL) []listingCandidate {
	type candidateWithNode struct {
		candidate listingCandidate
		node      *html.Node
	}

	byURL := make(map[string]candidateWithNode)

	walk(doc, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "a" {
			return
		}

		href := attr(node, "href")
		if !isListingDetailHref(href) {
			return
		}

		title := cleanText(textContent(node))
		if title == "" {
			return
		}

		card := nearestCardContainer(node)
		if card == nil {
			card = node.Parent
		}
		if card == nil {
			return
		}

		resolvedURL := resolveURL(baseURL, href)
		if resolvedURL == "" {
			return
		}

		byURL[resolvedURL] = candidateWithNode{
			candidate: listingCandidate{
				Title: title,
				URL:   resolvedURL,
			},
			node: card,
		}
	})

	candidates := make([]listingCandidate, 0, len(byURL))
	urls := make([]string, 0, len(byURL))
	for rawURL := range byURL {
		urls = append(urls, rawURL)
	}
	sort.Strings(urls)

	for _, rawURL := range urls {
		candidates = append(candidates, byURL[rawURL].candidate)
	}

	return candidates
}

func parseNextPageURL(doc *html.Node, baseURL *url.URL, currentPageURL string) string {
	currentURL, _ := url.Parse(currentPageURL)
	pageBaseURL := baseURL
	if currentURL != nil {
		pageBaseURL = currentURL
	}
	
	currentPage := 1
	if currentURL != nil {
		if value := currentURL.Query().Get("page"); value != "" {
			fmt.Sscanf(value, "%d", &currentPage)
		}
	}

	var nextURL string
	walk(doc, func(node *html.Node) {
		if nextURL != "" || node.Type != html.ElementNode || node.Data != "a" {
			return
		}

		href := strings.TrimSpace(attr(node, "href"))
		if href == "" {
			return
		}

		label := strings.ToLower(cleanText(textContent(node)))
		aria := strings.ToLower(strings.TrimSpace(attr(node, "aria-label")))
		rel := strings.ToLower(strings.TrimSpace(attr(node, "rel")))

		if strings.Contains(rel, "next") || strings.Contains(aria, "next") || label == "next" {
			nextURL = resolveURL(pageBaseURL, href)
			return
		}

		if !strings.Contains(href, "page=") {
			return
		}

		resolved := resolveURL(pageBaseURL, href)
		if resolved == "" {
			return
		}

		parsed, err := url.Parse(resolved)
		if err != nil {
			return
		}

		var page int
		if _, err := fmt.Sscanf(parsed.Query().Get("page"), "%d", &page); err == nil && page == currentPage+1 {
			nextURL = resolved
			return
		}

		if rePaginationNumber.MatchString(label) {
			var page int
			if _, err := fmt.Sscanf(label, "%d", &page); err == nil && page == currentPage+1 {
				nextURL = resolved
			}
		}
	})

	return nextURL
}

func extractJSONLDScripts(doc *html.Node, expectedType string) []string {
	var scripts []string
	walk(doc, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "script" {
			return
		}
		attrType := strings.ToLower(strings.TrimSpace(attr(node, "type")))
		if attrType != "application/ld+json" {
			return
		}

		if node.FirstChild == nil || node.FirstChild.Type != html.TextNode {
			return
		}

		raw := node.FirstChild.Data
		var wrapper struct {
			Type string `json:"@type"`
		}
		if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
			return
		}

		if expectedType == "" || strings.EqualFold(wrapper.Type, expectedType) {
			scripts = append(scripts, raw)
		}
	})

	return scripts
}

func walk(node *html.Node, fn func(*html.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walk(child, fn)
	}
}

func attr(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, key) {
			return attribute.Val
		}
	}
	return ""
}

func isListingDetailHref(href string) bool {
	href = strings.TrimSpace(href)
	return strings.Contains(href, "/jobs/") && !strings.Contains(href, "/job/")
}

func nearestCardContainer(node *html.Node) *html.Node {
	for current := node.Parent; current != nil; current = current.Parent {
		if current.Type != html.ElementNode {
			continue
		}
		switch current.Data {
		case "article", "li", "section", "div":
			lines := textLines(current)
			if len(lines) >= 2 && len(lines) <= 30 {
				return current
			}
		}
	}
	return nil
}

func firstHeadingText(root *html.Node, tag string) string {
	var value string
	walk(root, func(node *html.Node) {
		if value != "" || node.Type != html.ElementNode || node.Data != tag {
			return
		}
		value = cleanText(textContent(node))
	})
	return value
}

func extractDetailDescriptionHTML(doc *html.Node) string {
	var blocks []string

	walk(doc, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}

		switch strings.ToLower(node.Data) {
		case "h1", "h2", "h3", "header", "nav", "footer", "script", "style", "noscript":
			return
		}

		if node.Data == "div" || node.Data == "section" {
			class := attr(node, "class")
			if strings.Contains(strings.ToLower(class), "header") ||
				strings.Contains(strings.ToLower(class), "footer") ||
				strings.Contains(strings.ToLower(class), "nav") ||
				strings.Contains(strings.ToLower(class), "sidebar") {
				return
			}
		}

		if node.Data == "p" || node.Data == "li" {
			text := cleanText(textContent(node))
			if text == "" {
				return
			}
			if isMetaLine(text) || isStopLine(text) {
				return
			}
			blocks = append(blocks, text)
		}
	})

	return strings.TrimSpace(strings.Join(blocks, "\n"))
}

func textLines(node *html.Node) []string {
	raw := textContent(node)
	if raw == "" {
		return nil
	}

	lines := strings.Split(raw, "\n")
	cleaned := make([]string, 0, len(lines))
	seen := make(map[string]struct{})

	for _, line := range lines {
		line = cleanText(line)
		if line == "" {
			continue
		}
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		cleaned = append(cleaned, line)
	}

	return cleaned
}

func textContent(node *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)

	visit = func(current *html.Node) {
		if current == nil {
			return
		}
		if current.Type == html.ElementNode && (current.Data == "script" || current.Data == "style" || current.Data == "noscript") {
			return
		}
		if current.Type == html.TextNode {
			value := cleanText(current.Data)
			if value != "" {
				b.WriteString(value)
				b.WriteByte('\n')
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}

	visit(node)
	return b.String()
}

func cleanText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	value = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, value)
	value = reWhitespace.ReplaceAllString(value, " ")
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func externalIDFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	slug := path.Base(strings.TrimSuffix(parsed.Path, "/"))
	if slug == "." || slug == "/" {
		return ""
	}

	return slug
}

func normalizeListingPaths(paths []string) []string {
	seen := make(map[string]struct{})
	normalized := make([]string, 0, len(paths))

	for _, listingPath := range paths {
		listingPath = strings.TrimSpace(listingPath)
		if listingPath == "" {
			continue
		}
		if !strings.HasPrefix(listingPath, "/") {
			listingPath = "/" + listingPath
		}
		if _, ok := seen[listingPath]; ok {
			continue
		}
		seen[listingPath] = struct{}{}
		normalized = append(normalized, listingPath)
	}

	return normalized
}

func resolveURL(baseURL *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}

	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	if baseURL == nil {
		return ""
	}

	return baseURL.ResolveReference(parsed).String()
}

func (s *Scraper) resolveURL(href string) string {
	return resolveURL(s.baseURL, href)
}

func parseRelativePostedAt(value string, now time.Time) (time.Time, bool) {
	value = strings.ToLower(cleanText(value))
	if value == "" {
		return time.Time{}, false
	}

	switch value {
	case "new", "just posted":
		return now.UTC(), true
	}

	var amount int
	var unit string
	if _, err := fmt.Sscanf(value, "%d %s ago", &amount, &unit); err != nil {
		return time.Time{}, false
	}

	switch strings.TrimSuffix(unit, "s") {
	case "minute":
		return now.Add(-time.Duration(amount) * time.Minute).UTC(), true
	case "hour":
		return now.Add(-time.Duration(amount) * time.Hour).UTC(), true
	case "day":
		return now.AddDate(0, 0, -amount).UTC(), true
	case "week":
		return now.AddDate(0, 0, -7*amount).UTC(), true
	case "month":
		return now.AddDate(0, -amount, 0).UTC(), true
	default:
		return time.Time{}, false
	}
}

func isMetaLine(line string) bool {
	lower := strings.ToLower(cleanText(line))
	return strings.Contains(lower, "posted:") ||
		strings.Contains(lower, "apply by:") ||
		strings.Contains(lower, "deadline:")
}

func isStopLine(line string) bool {
	switch strings.ToLower(line) {
	case "report job", "easy apply", "share job post", "apply now", "save job", "similar jobs":
		return true
	default:
		return false
	}
}

func isLikelyWorkType(line string) bool {
	switch strings.ToLower(cleanText(line)) {
	case "full time", "part time", "contract", "internship", "temporary", "freelance":
		return true
	default:
		return false
	}
}

func isLikelyLocation(line string) bool {
	line = cleanText(line)
	if line == "" || len(strings.Fields(line)) > 6 {
		return false
	}
	lower := strings.ToLower(line)
	if isLikelyWorkType(line) || reRelativeDate.MatchString(line) {
		return false
	}
	return strings.Contains(lower, "remote") ||
		strings.Contains(lower, "nairobi") ||
		strings.Contains(lower, "mombasa") ||
		strings.Contains(lower, "kenya") ||
		strings.Contains(lower, "uganda") ||
		strings.Contains(lower, "tanzania") ||
		strings.Contains(lower, "rwanda") ||
		strings.Contains(lower, "africa")
}