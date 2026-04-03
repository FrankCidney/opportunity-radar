package normalize

func applySourceOverrides(raw RawJob, job NormalizedJob) NormalizedJob {
	switch raw.Source {
	case "remotive":
		job.Description = normalizeRemotiveDescription(raw.Description)
	}

	return job
}
