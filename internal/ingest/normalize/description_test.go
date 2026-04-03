package normalize

import "testing"

func TestNormalizeRemotiveDescriptionPrependsMetadataAndCleansText(t *testing.T) {
	raw := RawJob{
		JobType: "full_time",
		Salary:  "$30k - $40k",
		Description: `
<p>Looking for a freelance opportunity   where you can make an impact?</p>
<p>*A Day in the Life of a Personalized Internet Assessor:*</p>
<ul>
  <li>Analyze and provide feedback on texts, pages, images</li>
  <li>Review and rate search results for relevance and quality</li>
</ul>
`,
	}

	got := normalizeRemotiveDescription(raw)
	want := "Job type: Full Time\n" +
		"Salary: $30k - $40k\n\n" +
		"Looking for a freelance opportunity where you can make an impact?\n\n" +
		"A Day in the Life of a Personalized Internet Assessor:\n\n" +
		"- Analyze and provide feedback on texts, pages, images\n" +
		"- Review and rate search results for relevance and quality"

	if got != want {
		t.Fatalf("unexpected normalized description:\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}
