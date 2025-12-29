### Essential 
```bash
gix init   
gix status 
```

### Commit Management
```bash
gix commit
gix commit / gix c
gix amend
gix reword
gix reword [oldhash-commit] [new message commit:"feat(auth): add email and password validation"]
gix uncommit
gix bisect
gix gc / gix prune
gix clone <repo>
gix remote
```

### Branch Operations
```bash
gix branch OR gix br
gix protect-branch <branch>
gix branch <name>  OR gix br <name>
gix branch --tree
gix switch <branch> OR gix sw <branch>
gix rename <old> <new>
gix delete <branch> OR gix <branch> -d
gix cleanup
gix cleanup --dry-run
```

### History & Navigation
```bash
gix log 
gix log --graph
gix back <n>
gix goto <commit>
# Example:
gix goto 4b3a2f1
gix goto HEAD~5
gix goto yesterday
gix goto @{2.days.ago}
####
gix undo
```

### Merge & Conflicts
```bash
gix merge <branch>
gix rebase <branch>
gix conflicts
```

### Stash
```bash
gix stash 
gix stash list
gix stash pop
gix stash apply <index>
```

### Security & GPG
```bash
gix gpg setup
gix gpg list
gix gpg sign
gix gpg verify
gix gpg export
```

### Hooks & Automation
```bash
gix hooks install
gix hooks init
gix hooks enable
gix hooks add <hook> <command>
gix hooks list 
gix hooks remove <hook> <index>
```

### Commit Linting
```bash
gix lint   
gix lint --fix 
gix lint config
```

### Release & Changelog
```bash
gix release
gix release --preview   
gix release --patch
gix release --major
gix changelog  
gix changelog --update  
```

### Diff & Comparison
```bash
gix diff   
gix diff <branch>  
gix diff --stat
```

### Push & Pull
```bash
gix push   
gix push --force   
gix pull
gix fetch
gix reset
gix sync   
```

### Tags
```bash
gix tag <name> 
gix tag list   
gix tag delete <name>  OR gix tag <name> -d   
```

### Configuration
```bash
gix config 
gix config edit 
```

### Utilities
```bash
gix clean  
```

### AI & Automation
```bash
gix interactive OR gix i
```

### Help
```bash
gix help   
gix help <command> 
gix --version  
```