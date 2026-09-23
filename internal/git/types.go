package git

// Status summarizes the working tree and current branch state, as used
// by `gix status`. It's a value type so commands can format it however
// they like without re-querying git multiple times.
type Status struct {
	Head    HeadState // full HEAD state: on-branch / detached / unborn
	Branch  string    // "" if HEAD is detached or unborn; mirrors Head.Branch for convenience
	Clean   bool
	Ahead   int // commits current branch has that Base doesn't
	Behind  int // commits Base has that current branch doesn't
	Base    string
	HasBase bool // false when Base comparison wasn't possible (e.g. base branch missing, or HEAD isn't on a branch)
}

// BranchRef is one local branch as returned by ListBranches, carrying
// enough detail for `gix list` and `gix branch --tree` to render a
// tree view without extra git calls per branch.
type BranchRef struct {
	Name    string
	Current bool
}
