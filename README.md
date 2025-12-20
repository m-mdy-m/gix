# Gix

A modern Git wrapper that makes version control simple, safe, and intuitive.

## About

Gix is a comprehensive Git wrapper designed to simplify everyday Git operations. It provides intelligent defaults, safety checks, and automation while maintaining full compatibility with Git. Think of it as Git with guardrails and smart assistance.

The project started from a simple observation: Git is powerful but complex. Developers spend too much time wrestling with Git commands instead of focusing on their code. Gix aims to bridge this gap by providing a more user-friendly interface without sacrificing Git's power and flexibility.

## Why Gix?

Git is an incredible tool, but its learning curve is steep and its interface can be unforgiving. Over the years, we've seen developers struggle with common tasks like commit message formatting, branch management, and merge conflicts. Gix addresses these pain points by:

Providing smart commit workflows with automatic analysis and conventional commit formatting. The tool examines your changes and suggests appropriate commit types and messages, making it easy to maintain a clean commit history.

Offering safe operations with automatic backups before dangerous commands. When you're about to rebase, force push, or perform any operation that could lose work, Gix automatically creates a safety backup. This gives you confidence to experiment and learn without fear.

Simplifying branch management with clear visualizations and intelligent cleanup. Gix helps you understand branch relationships, identifies stale branches, and makes it easy to keep your repository organized.

Standardizing team workflows through built-in linting and hooks management. Set up commit linting, pre-commit checks, and other quality gates with simple commands that work consistently across your team.

## Features

Gix provides intelligent commit management that analyzes your changes and guides you through creating well-formatted commits. It supports conventional commits out of the box and can even suggest commit messages based on your code changes.

The branch management system gives you clear visibility into your repository structure. You can see which branches are active, which are stale, and how they relate to each other. Operations like creating, switching, and cleaning up branches become straightforward.

Safety is built into every operation. Gix creates automatic backups before potentially destructive operations, provides clear warnings, and offers easy undo functionality. You can experiment with Git operations knowing you can always recover.

Release management and changelog generation are automated based on your commit history. If you follow conventional commits, Gix can automatically determine version numbers and generate detailed changelogs.

GPG signing and security features are simplified with guided setup wizards. Gix makes it easy to sign commits and verify signatures, enhancing your repository's security.

The tool includes comprehensive hooks management with pre-configured templates for common use cases. Setting up lint-staged, commitlint, and other quality checks takes just one command.

## Installation

### From Source

Clone the repository and build:
```bash
git clone https://github.com/m-mdy-m/gix.git
cd gix
make install
```

This will build the binary and install it to your Go bin directory.

### Manual Installation

Download the latest release from the releases page, extract it, and move the binary to a location in your PATH:
```bash
chmod +x gix
sudo mv gix /usr/local/bin/
```

## Quick Start

Initialize a new repository with Gix standards:
```bash
gix init
```

This sets up Git, creates configuration files, installs hooks, and configures commit linting.

Make some changes and commit with intelligent assistance:
```bash
gix commit
```

Gix will analyze your changes, suggest a commit type and message, run pre-commit checks, and guide you through the commit process.

Create and work on a feature branch:
```bash
gix branch feature/user-authentication
# Make your changes
gix commit
gix pr-ready
```

The pr-ready command checks for conflicts, validates commits, runs tests, and confirms your branch is ready for a pull request.

## Configuration

Gix works with sensible defaults but can be customized. View your current configuration:
```bash
gix config
```

Common settings include commit signing preferences, default branch names, hook configurations, and linting rules. All settings can be modified with the config command.

## Documentation

For detailed documentation on all commands and features, visit the documentation site or use the built-in help:
```bash
gix help
gix help <command>
```

## Development

Gix is written in Go and uses a Makefile for common development tasks. To get started with development:
```bash
git clone https://github.com/m-mdy-m/gix.git
cd gix
make dev
```

Run tests with:
```bash
make test
```

Build for all platforms:
```bash
make build-all
```

## Contributing

We welcome contributions of all kinds, from bug reports to feature implementations. Please read [CONTRIBUTING.md](./CONTRIBUTING.md) for details on our development process and how to submit pull requests.

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.

## Contact

For questions, suggestions, or issues, please open an issue on GitHub or email us at bitsgenix@gmail.com.

## Acknowledgments

Gix builds on the shoulders of giants. We're grateful to the Git project and the many tools in the Git ecosystem that have inspired this work. Special thanks to the contributors of conventional commits, commitlint, husky, and other projects that have shaped modern Git workflows.