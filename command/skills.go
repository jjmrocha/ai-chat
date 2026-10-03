package command

// SkillsController is the skill catalog /skills reads. Pass one to [Skills], or
// to chat.WithSkills, which wires it for you.
type SkillsController interface {
	// Skills lists the names of the registered skills.
	Skills() []string
}

type skillsCmd struct{ coll SkillsController }

// Skills returns the /skills command, which lists the skills in coll.
func Skills(coll SkillsController) Command {
	return skillsCmd{coll: coll}
}

func (skillsCmd) Name() string {
	return "skills"
}

func (skillsCmd) Help() string {
	return "List available skills"
}

func (c skillsCmd) Run(ctx Context, _ string) {
	printList(ctx, "Skills", "No skills registered.", c.coll.Skills())
}

type skillCmd struct{ name, description string }

// SkillCommand returns a command named name that sends its input to the agent
// as a user turn, exactly as typed, so the agent can pick up the skill. /help
// and completion show description.
func SkillCommand(name, description string) Command {
	return skillCmd{name: name, description: description}
}

func (c skillCmd) Name() string {
	return c.name
}

func (c skillCmd) Help() string {
	return c.description
}

func (skillCmd) Prompt() bool {
	return true
}

func (skillCmd) Run(Context, string) {}
