package command

type compactCmd struct{}

// Compact returns the /compact command: force context compaction. The outcome
// is reported through the agent's feedback events, not by this command.
func Compact() Command                       { return compactCmd{} }
func (compactCmd) Name() string              { return "compact" }
func (compactCmd) Help() string              { return "/compact        Force context compaction" }
func (compactCmd) Run(ctx Context, _ string) { ctx.Agent().Compact() }
