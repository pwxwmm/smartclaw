package chain

func defaultPatterns() []MergePattern {
	return []MergePattern{
		{
			Sequence: []string{"read_file", "edit_file"},
			MergedAs: "edit_file",
			Reason:   "read-then-edit: reading file first, then editing it — can combine into single edit_file with target content",
			Savings:  1,
		},
		{
			Sequence: []string{"read_file", "read_file"},
			MergedAs: "bash",
			Reason:   "multi-read: reading multiple files can be combined into a single cat/glob command",
			Savings:  1,
		},
		{
			Sequence: []string{"bash", "bash"},
			MergedAs: "bash",
			Reason:   "sequential commands: can be combined with && or ; operator",
			Savings:  1,
		},
		{
			Sequence: []string{"bash", "edit_file"},
			MergedAs: "execute_code",
			Reason:   "command-then-edit: running a command then editing a file can be collapsed into execute_code",
			Savings:  1,
		},
		{
			Sequence: []string{"edit_file", "bash"},
			MergedAs: "execute_code",
			Reason:   "edit-then-test: editing then running tests can be collapsed into execute_code",
			Savings:  1,
		},
		{
			Sequence: []string{"glob", "read_file"},
			MergedAs: "bash",
			Reason:   "find-then-read: glob to find files, then reading — combine into find ... -exec cat",
			Savings:  1,
		},
		{
			Sequence: []string{"grep", "read_file"},
			MergedAs: "bash",
			Reason:   "search-then-read: grep to find, then reading matching files — combine into grep + cat",
			Savings:  1,
		},
	}
}
