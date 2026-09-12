package command

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

func (c skillsCmd) Run(ctx Context, args string) {
	printList(ctx, "Skills", "No skills registered.", c.coll.Skills())
}
