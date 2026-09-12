package command

type compactCmd struct{}

// Compact returns the /compact command, which forces context compaction. The
// outcome is reported in the transcript, not returned.
func Compact() Command {
	return compactCmd{}
}

func (compactCmd) Name() string {
	return "compact"
}

func (compactCmd) Help() string {
	return "Force context compaction"
}

func (compactCmd) Run(ctx Context, _ string) {
	ctx.Agent().Compact()
}
