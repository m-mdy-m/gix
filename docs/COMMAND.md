### Essential 
```bash
gix init   
gix status 
```

### Commit Management
```bash
gix commit
gix amend
gix reword
gix reword [oldhash-commit] [new message commit:"feat(auth): add email and password validation"]
```

### Branch Operations
```bash
gix list # list branches
gix <type> start branch
gix <type> finish # finish current branch
gix branch --tree

```

### History & Navigation
```bash
gix goto <commit>
# Example:
gix goto 4b3a2f1
gix goto HEAD~5
####
gix undo
```

### Merge 
```bash
gix merge
gix rebase
gix squash
```

### Stash
```bash
gix stash 
gix stash list
gix stash pop
gix stash apply <index>
```

### Hooks & Automation
```bash
gix hooks init
gix hooks add <hook> <command>
gix hooks list 
gix hooks remove <hook> <index>
```

### Release & Changelog
```bash
gix release
gix release --preview
gix release --patch
gix release --major
```
### Config
```bash
gix config 
```
### Diff & Comparison
```bash
gix diff <branch>  
gix diff --stat
```

### Push & Pull
```bash
gix pub
gix sync
```

### Help
```bash
gix help   
gix help <command> 
gix --version  
```