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
