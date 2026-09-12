package command

// Quitter ends the session. chat.Chat implements it, so [Exit] is registered
// for you.
type Quitter interface {
	// Quit asks the front-end to shut down.
	Quit()
}

type exitCmd struct{ q Quitter }

// Exit returns the /exit command, which ends the session through q. It is
// registered automatically; pass your own through chat.WithCommand to replace
// it.
func Exit(q Quitter) Command {
	return exitCmd{q: q}
}

func (exitCmd) Name() string {
	return "exit"
}

func (exitCmd) Help() string {
	return "Quit"
}

func (c exitCmd) Run(Context, string) {
	c.q.Quit()
}
