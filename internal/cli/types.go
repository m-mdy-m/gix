package cli

import (
	"github.com/m-mdy-m/gix/internal/config"
	"github.com/m-mdy-m/gix/internal/git"
	"github.com/spf13/cobra"
)

type Context struct {
	root string
	cfg  *config.Config
}

func (c *Context) Repo() (string, error) {
	if c.root != "" {
		return c.root, nil
	}
	if !git.IsRepo() {
		return "", errNotARepo
	}
	root, err := git.RepoRoot()
	if err != nil {
		return "", err
	}
	c.root = root
	return root, nil
}

func (c *Context) Config() (string, *config.Config, error) {
	root, err := c.Repo()
	if err != nil {
		return "", nil, err
	}
	if c.cfg == nil {
		cfg, err := config.Load(root)
		if err != nil {
			return "", nil, err
		}
		c.cfg = cfg
	}
	return root, c.cfg, nil
}

var errNotARepo = notARepoError{}

type notARepoError struct{}

func (notARepoError) Error() string {
	return "not a git repository (or any parent up to mount point)"
}

type Registrable interface {
	Register(root *cobra.Command, ctx *Context)
}

type RegistrableFunc func(root *cobra.Command, ctx *Context)

func (f RegistrableFunc) Register(root *cobra.Command, ctx *Context) { f(root, ctx) }
