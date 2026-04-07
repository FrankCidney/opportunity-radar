package normalize

import "testing"

func TestNormalizeBrighterMondayDescriptionPrependsMetadataAndCleansText(t *testing.T) {
	raw := RawJob{
		Source:   "brightermonday",
		JobType:  "Full Time",
		Salary:   "KSh 45,000 - 60,000",
		Location: "Mombasa",
		Description: `
We are looking for a hands-on ERPNext Technical Associate.

* Assist with configuration and system testing
* Document workflows and integrations
`,
		RawData: map[string]interface{}{
			"experience_level":  "Entry level",
			"experience_length": "1 year",
			"qualification":     "Bachelors",
		},
	}

	got := normalizeBrighterMondayDescription(raw)
	want := "Job type: Full Time\n" +
		"Location: Mombasa\n" +
		"Salary: KSh 45,000 - 60,000\n" +
		"Experience level: Entry level\n" +
		"Experience length: 1 year\n" +
		"Qualification: Bachelors\n\n" +
		"We are looking for a hands-on ERPNext Technical Associate.\n\n" +
		"- Assist with configuration and system testing\n" +
		"- Document workflows and integrations"

	if got != want {
		t.Fatalf("unexpected normalized description:\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}
