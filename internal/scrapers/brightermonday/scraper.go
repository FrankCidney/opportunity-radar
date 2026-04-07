package brightermonday

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
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
	sourceID            = "brightermonday"
	defaultBaseURL      = "https://www.brightermonday.co.ke"
	defaultMaxPages     = 3
	defaultHTTPTimeout  = 15 * time.Second
	defaultRequestDelay = 1 * time.Second
	defaultUserAgent    = "opportunity-radar/1.0 (+https://github.com/FrankCidney/opportunity-radar.git)"
)

var (
	reWhitespace       = regexp.MustCompile(`\s+`)
	reSalary           = regexp.MustCompile(`(?i)(ksh\b|kes\b|\$\s?\d|salary\s*:)`)
	reRelativeDate     = regexp.MustCompile(`(?i)^(new|just posted|\d+\s+(minute|minutes|hour|hours|day|days|week|weeks|month|months)\s+ago)$`)
	rePaginationNumber = regexp.MustCompile(`^\d+$`)
)

var metadataLabels = []string{
	"Min Qualification",
	"Experience Level",
	"Experience Length",
	"Working Hours",
	"Salary",
	"Industry",
	"Job category",
	"Town",
	"Country",
	"Location",
}

type Config struct {
	BaseURL         string
	ListingPaths    []string
	MaxPagesPerPath int
	RequestDelay    time.Duration
}

type Scraper struct {
	baseURL       *neturl.URL
	listingPaths  []string
	maxPages      int
	requestDelay  time.Duration
	client        *http.Client
	logger        *slog.Logger
	requestMu     sync.Mutex
	lastRequestAt time.Time
}

type listingCandidate struct {
	Title       string
	Company     string
	URL         string
	Location    string
	WorkType    string
	Salary      string
	RelativeAge string
	Category    string
	CompanyLogo string
}

type detailJob struct {
	Title            string
	Company          string
	Description      string
	Location         string
	WorkType         string
	Salary           string
	PostedAt         time.Time
	ExperienceLevel  string
	ExperienceLength string
	Qualification    string
	CompanyLogo      string
	RawData          map[string]interface{}
}

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

	parsedBase, err := neturl.Parse(baseURL)
	if err != nil {
		parsedBase, _ = neturl.Parse(defaultBaseURL)
	}

	listingPaths := normalizeListingPaths(cfg.ListingPaths)
	if len(listingPaths) == 0 {
		listingPaths = []string{"/jobs/software-data"}
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

	raws := make([]normalize.RawJob, 0, len(discovered))
	urls := make([]string, 0, len(discovered))
	for rawURL := range discovered {
		urls = append(urls, rawURL)
	}
	sort.Strings(urls)

	for _, rawURL := range urls {
		candidate := discovered[rawURL]
		detail, err := s.fetchDetail(ctx, candidate.URL)
		if err != nil {
			s.logger.Warn("failed to fetch BrighterMonday detail page", "url", candidate.URL, "error", err)
			continue
		}

		raws = append(raws, s.toRawJob(candidate, detail))
	}

	s.logger.Info("brightermonday scrape complete", "count", len(raws))
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

		if parsedNextURL == nextURL {
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
	location := firstNonEmpty(detail.Location, candidate.Location)
	workType := firstNonEmpty(detail.WorkType, candidate.WorkType)
	salary := firstNonEmpty(detail.Salary, candidate.Salary)
	companyLogo := firstNonEmpty(detail.CompanyLogo, candidate.CompanyLogo)

	rawData := map[string]interface{}{
		"category": candidate.Category,
	}

	for key, value := range detail.RawData {
		rawData[key] = value
	}

	if candidate.RelativeAge != "" {
		rawData["listing_relative_age"] = candidate.RelativeAge
	}
	if detail.ExperienceLevel != "" {
		rawData["experience_level"] = detail.ExperienceLevel
	}
	if detail.ExperienceLength != "" {
		rawData["experience_length"] = detail.ExperienceLength
	}
	if detail.Qualification != "" {
		rawData["qualification"] = detail.Qualification
	}

	return normalize.RawJob{
		Source:      s.Source(),
		Title:       title,
		Company:     company,
		CompanyLogo: companyLogo,
		ExternalID:  externalIDFromURL(candidate.URL),
		Location:    location,
		JobType:     workType,
		Salary:      salary,
		Description: detail.Description,
		URL:         candidate.URL,
		PostedAt:    detail.PostedAt.UTC().Format(time.RFC3339),
		RawData:     rawData,
	}
}

func parseListingPage(doc *html.Node, baseURL *neturl.URL, pageURL string) ([]listingCandidate, string) {
	currentURL, _ := neturl.Parse(pageURL)

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

		lines := textLines(card)
		candidate := parseListingCandidateFromLines(lines, title)
		candidate.Title = title
		candidate.URL = resolvedURL
		candidate.CompanyLogo = extractImageSource(card, baseURL)

		byURL[resolvedURL] = candidateWithNode{candidate: candidate, node: card}
	})

	candidates := make([]listingCandidate, 0, len(byURL))
	urls := make([]string, 0, len(byURL))
	for rawURL := range byURL {
		urls = append(urls, rawURL)
	}
	sort.Strings(urls)

	for _, rawURL := range urls {
		item := byURL[rawURL]
		candidates = append(candidates, item.candidate)
	}

	return candidates, parseNextPageURL(doc, baseURL, currentURL)
}

func parseDetailPage(doc *html.Node, baseURL *neturl.URL, pageURL string) (detailJob, error) {
	title := firstHeadingText(doc, "h1")
	if title == "" {
		return detailJob{}, fmt.Errorf("missing job title")
	}

	lines := textLines(doc)
	if len(lines) == 0 {
		return detailJob{}, fmt.Errorf("missing page text")
	}

	detail := detailJob{
		Title:       title,
		Company:     firstHeadingText(doc, "h2"),
		CompanyLogo: extractImageSource(doc, baseURL),
		RawData:     map[string]interface{}{},
	}

	topLines := lines
	if len(topLines) > 30 {
		topLines = topLines[:30]
	}

	for _, line := range topLines {
		if detail.PostedAt.IsZero() {
			if postedAt, ok := parseRelativePostedAt(line, time.Now().UTC()); ok {
				detail.PostedAt = postedAt
			}
		}

		if detail.Salary == "" && reSalary.MatchString(line) {
			detail.Salary = cleanSalary(line)
		}

		if detail.WorkType == "" && isLikelyWorkType(line) {
			detail.WorkType = line
		}

		if detail.Location == "" && isLikelyLocation(line) {
			detail.Location = line
		}
	}

	metadata := parseDetailMetadata(lines)
	detail.ExperienceLevel = metadata["experience level"]
	detail.ExperienceLength = metadata["experience length"]
	detail.Qualification = firstNonEmpty(metadata["minimum qualification"], metadata["min qualification"])

	if detail.WorkType == "" {
		detail.WorkType = firstNonEmpty(metadata["working hours"], metadata["job type"])
	}
	if detail.Location == "" {
		detail.Location = metadata["location"]
	}
	if detail.Salary == "" {
		detail.Salary = metadata["salary"]
	}

	detail.Description = buildDetailDescription(lines)
	if detail.Description == "" {
		return detailJob{}, fmt.Errorf("missing description")
	}
	if detail.PostedAt.IsZero() {
		detail.PostedAt = time.Now().UTC()
	}

	detail.RawData["source_page"] = strings.TrimSpace(pageURL)
	if metadata["industry"] != "" {
		detail.RawData["industry"] = metadata["industry"]
	}

	return detail, nil
}

func parseListingCandidateFromLines(lines []string, title string) listingCandidate {
	candidate := listingCandidate{}
	trimmedTitle := cleanText(title)

	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" || line == trimmedTitle || isGenericListingLine(line) {
			continue
		}
		filtered = append(filtered, line)
	}

	if len(filtered) > 0 {
		candidate.Company = filtered[0]
	}

	for _, line := range filtered[1:] {
		switch {
		case candidate.RelativeAge == "" && reRelativeDate.MatchString(line):
			candidate.RelativeAge = line
		case candidate.Salary == "" && reSalary.MatchString(line):
			candidate.Salary = cleanSalary(line)
			location, workType := splitLocationAndWorkType(line)
			if candidate.Location == "" {
				candidate.Location = location
			}
			if candidate.WorkType == "" {
				candidate.WorkType = workType
			}
		case candidate.WorkType == "" && isLikelyWorkType(line):
			candidate.WorkType = line
		case candidate.Location == "" && isLikelyLocation(line):
			candidate.Location = line
		case candidate.Category == "" && isLikelyCategory(line):
			candidate.Category = line
		}
	}

	return candidate
}

func parseNextPageURL(doc *html.Node, baseURL *neturl.URL, currentURL *neturl.URL) string {
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

		href := attr(node, "href")
		if href == "" {
			return
		}

		label := strings.ToLower(cleanText(textContent(node)))
		aria := strings.ToLower(strings.TrimSpace(attr(node, "aria-label")))
		rel := strings.ToLower(strings.TrimSpace(attr(node, "rel")))

		if strings.Contains(rel, "next") || strings.Contains(aria, "next") || label == "next" {
			nextURL = resolveURL(baseURL, href)
			return
		}

		if !strings.Contains(href, "page=") {
			return
		}

		resolved := resolveURL(baseURL, href)
		if resolved == "" {
			return
		}

		parsed, err := neturl.Parse(resolved)
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

func parseDetailMetadata(lines []string) map[string]string {
	metadata := make(map[string]string)
	for _, line := range lines {
		extracted := extractMetadataFields(line)
		if len(extracted) > 0 {
			for key, value := range extracted {
				metadata[key] = value
			}
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(cleanText(parts[0]))
		value := cleanText(parts[1])
		if key == "" || value == "" {
			continue
		}

		metadata[key] = value
	}
	return metadata
}

func extractMetadataFields(line string) map[string]string {
	line = cleanText(line)
	if line == "" {
		return nil
	}

	lower := strings.ToLower(line)
	type fieldPosition struct {
		key   string
		start int
		end   int
	}

	positions := make([]fieldPosition, 0, len(metadataLabels))
	for _, label := range metadataLabels {
		token := strings.ToLower(label) + ":"
		index := strings.Index(lower, token)
		if index < 0 {
			continue
		}
		positions = append(positions, fieldPosition{
			key:   strings.ToLower(cleanText(label)),
			start: index,
			end:   index + len(token),
		})
	}

	if len(positions) == 0 {
		return nil
	}

	sort.Slice(positions, func(i, j int) bool {
		return positions[i].start < positions[j].start
	})

	fields := make(map[string]string, len(positions))
	for i, position := range positions {
		valueEnd := len(line)
		if i+1 < len(positions) {
			valueEnd = positions[i+1].start
		}

		value := cleanText(line[position.end:valueEnd])
		if value != "" {
			fields[position.key] = value
		}
	}

	return fields
}

func buildDetailDescription(lines []string) string {
	var blocks []string
	collecting := false

	for _, line := range lines {
		switch normalizeHeadingKey(line) {
		case "job summary", "job description requirements", "job descriptions requirements":
			collecting = true
			continue
		case "important safety tips", "share job post", "similar jobs", "stay updated", "log in to apply now", "share link":
			collecting = false
		}

		if !collecting || line == "" || isMetaLine(line) {
			continue
		}

		if isStopLine(line) {
			break
		}

		blocks = append(blocks, line)
	}

	return strings.TrimSpace(strings.Join(blocks, "\n"))
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

func externalIDFromURL(rawURL string) string {
	parsed, err := neturl.Parse(rawURL)
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

func resolveURL(baseURL *neturl.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}

	parsed, err := neturl.Parse(href)
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

func extractImageSource(node *html.Node, baseURL *neturl.URL) string {
	var source string
	walk(node, func(current *html.Node) {
		if source != "" || current.Type != html.ElementNode || current.Data != "img" {
			return
		}
		src := attr(current, "src")
		if src == "" {
			src = attr(current, "data-src")
		}
		if src == "" {
			return
		}
		source = resolveURL(baseURL, src)
	})
	return source
}

func nearestCardContainer(node *html.Node) *html.Node {
	for current := node.Parent; current != nil; current = current.Parent {
		if current.Type != html.ElementNode {
			continue
		}
		switch current.Data {
		case "article", "li", "section", "div":
			lines := textLines(current)
			if len(lines) >= 3 && len(lines) <= 20 {
				return current
			}
		}
	}
	return nil
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

func walk(node *html.Node, fn func(*html.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walk(child, fn)
	}
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
	return strings.Contains(href, "/listings/")
}

func isGenericListingLine(line string) bool {
	switch strings.ToLower(line) {
	case "easy apply", "set alert", "apply now", "search", "filter results":
		return true
	default:
		return false
	}
}

func cleanSalary(line string) string {
	line = cleanText(line)
	if line == "" {
		return ""
	}

	if idx := strings.Index(strings.ToLower(line), "salary:"); idx >= 0 {
		return cleanText(line[idx+len("salary:"):])
	}

	if idx := strings.Index(strings.ToLower(line), "ksh"); idx >= 0 {
		return cleanText(line[idx:])
	}

	return line
}

func splitLocationAndWorkType(line string) (string, string) {
	line = cleanText(line)
	if line == "" {
		return "", ""
	}

	workTypes := []string{"Full Time", "Part Time", "Contract", "Internship", "Temporary"}
	for _, workType := range workTypes {
		index := strings.Index(strings.ToLower(line), strings.ToLower(workType))
		if index >= 0 {
			location := cleanText(line[:index])
			return location, workType
		}
	}

	return line, ""
}

func isLikelyWorkType(line string) bool {
	switch strings.ToLower(cleanText(line)) {
	case "full time", "part time", "contract", "internship", "temporary":
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
	if isLikelyWorkType(line) || reRelativeDate.MatchString(line) || reSalary.MatchString(line) {
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

func isLikelyCategory(line string) bool {
	lower := strings.ToLower(cleanText(line))
	if lower == "" || len(strings.Fields(lower)) > 5 {
		return false
	}

	return strings.Contains(lower, "software") ||
		strings.Contains(lower, "data") ||
		strings.Contains(lower, "product") ||
		strings.Contains(lower, "design")
}

func isMetaLine(line string) bool {
	lower := strings.ToLower(cleanText(line))
	return strings.Contains(lower, "min qualification:") ||
		strings.Contains(lower, "minimum qualification:") ||
		strings.Contains(lower, "experience level:") ||
		strings.Contains(lower, "experience length:") ||
		strings.Contains(lower, "working hours:")
}

func isStopLine(line string) bool {
	switch line {
	case "Report Job", "Easy Apply":
		return true
	default:
		return false
	}
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

func normalizeHeadingKey(value string) string {
	value = cleanText(strings.ToLower(value))
	value = strings.ReplaceAll(value, "&", " ")
	value = strings.ReplaceAll(value, "/", " ")
	value = strings.ReplaceAll(value, "-", " ")
	value = reWhitespace.ReplaceAllString(value, " ")
	return strings.TrimSpace(value)
}
