package command

import "strings"

type skillsCmd struct{ coll SkillsController }

// Skills returns the /skills command bound to coll: list the available skills.
func Skills(coll SkillsController) Command { return skillsCmd{coll: coll} }
func (skillsCmd) Name() string             { return "skills" }
func (skillsCmd) Help() string             { return "/skills         List available skills" }
func (c skillsCmd) Run(ctx Context, args string) {
	names := c.coll.Skills()
	if len(names) == 0 {
		ctx.Print(Info, "No skills registered.")
		return
	}

	lines := make([]string, 0, len(names)+1)
	lines = append(lines, "Skills:")
	for _, name := range names {
		lines = append(lines, "  "+name)
	}
	ctx.Print(Info, strings.Join(lines, "\n"))
}
