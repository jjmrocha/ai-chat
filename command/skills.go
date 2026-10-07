package command

// SkillCatalog is the skill collection /skills lists. Pass one to [Skills], or
// to chat.WithSkills, which wires it for you.
type SkillCatalog interface {
	// Skills lists the names of the registered skills.
	Skills() []string
}

type skillsCmd struct{ coll SkillCatalog }

// Skills returns the /skills command, which lists the skills in coll.
func Skills(coll SkillCatalog) Command {
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
