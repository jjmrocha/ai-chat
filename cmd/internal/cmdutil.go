// Package cmdutil holds helpers shared by the example agents under cmd/.
package cmdutil

import (
	"os"
	"path/filepath"

	"github.com/jjmrocha/ai-toolkit/skills"
)

// SkillsRoot is the directory under the user's home where example skills live.
const SkillsRoot = ".claude/skills"

// AddSkills loads the named skills from the user's home skills directory into
// the collection.
func AddSkills(coll *skills.Collection, names ...string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	for _, name := range names {
		if err := coll.Add(filepath.Join(home, SkillsRoot, name)); err != nil {
			return err
		}
	}

	return nil
}
