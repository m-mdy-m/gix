# Contributing to Gix

Thank you for your interest in contributing to Gix. This document provides guidelines and information for contributors.

## Getting Started

Before you start contributing, take some time to familiarize yourself with the project. Read through the README, try out the commands, and understand what Gix is trying to achieve. The goal is to make Git more accessible without hiding its power.

Fork the repository on GitHub and clone your fork locally. Set up the upstream remote so you can keep your fork synchronized:
```bash
git clone https://github.com/m-mdy-m/gix.git
cd gix
git remote add upstream https://github.com/originalowner/gix.git
```

## Development Setup

Gix requires Go 1.25 or later. Make sure you have it installed and properly configured. The project uses a Makefile for common tasks, which simplifies the development workflow.

Install development dependencies:
```bash
make dev
```

This sets up everything you need for development, including linters and testing tools.

Build the project:
```bash
make build
```

Run the test suite:
```bash
make test
```

For integration tests:
```bash
make test-integration
```

## Project Structure

The codebase is organized to be easy to navigate. The cmd directory contains command implementations. The pkg directory contains reusable packages and core logic. The internal directory contains private packages. Tests live alongside the code they test.

Understanding this structure will help you find your way around and know where to add new code.

## Making Changes

When you're ready to make changes, create a new branch from main. Use descriptive branch names that indicate what you're working on:
```bash
git checkout -b feature/add-squash-command
git checkout -b fix/merge-conflict-detection
```

Make your changes in logical, focused commits. Each commit should represent a single logical change. Write clear commit messages following the conventional commits format that Gix itself encourages.

Run the tests frequently to catch issues early:
```bash
make test
```

Before submitting, make sure your code passes all checks:
```bash
make check
```

This runs linters, formatters, and all tests.

## Code Style

We follow standard Go conventions and idioms. Code should be formatted with gofmt. Use golangci-lint for additional checks. Write clear, self-documenting code with comments where necessary.

Error handling should be explicit and informative. User-facing messages should be clear and helpful. Consider what information would help someone debug an issue.

## Testing

Tests are crucial for maintaining reliability. Write unit tests for new functionality. Add integration tests for command workflows. Test error conditions and edge cases, not just happy paths.

Good tests are readable and maintainable. They should clearly show what's being tested and why. Avoid testing implementation details; focus on behavior.

## Documentation

Documentation is as important as code. Update the README if you add user-facing features. Document new commands and options. Add examples that show how to use new functionality.

Code comments should explain why, not what. The code itself should be clear enough to show what it does. Comments should provide context and reasoning.

## Submitting Changes

When your changes are ready, push your branch to your fork:
```bash
git push origin feature/your-feature
```

Open a pull request on GitHub. Provide a clear description of what you changed and why. Reference any related issues. Include examples of the new functionality if applicable.

Your PR should have passing tests, updated documentation if needed, clear commit messages, and focused changes that address a single concern.

## Code Review

All contributions go through code review. This is a collaborative process aimed at maintaining code quality and sharing knowledge. Reviewers may ask questions, suggest changes, or request additional tests.

Be open to feedback and willing to iterate. Reviews are not personal critiques but discussions about how to make the code better. Similarly, when reviewing others' code, be constructive and respectful.

## Release Process

Releases are managed by maintainers. Version numbers follow semantic versioning. The changelog is generated from commit messages, which is why conventional commits are important.

## Getting Help

If you have questions or need help:

Open an issue on GitHub for bugs or feature requests. Use discussions for questions about development or design. Email bitsgenix@gmail.com for other inquiries.

## Community Guidelines

We strive to maintain a welcoming and inclusive community. Be respectful and considerate in all interactions. Focus on constructive feedback. Assume good intentions. Help others learn and grow.

Everyone was a beginner once. Be patient with newcomers and remember that different people bring different perspectives and strengths to the project.

## Recognition

Contributors are recognized in the project's contributor list. Significant contributions will be acknowledged in release notes. We appreciate all forms of contribution, from code to documentation to bug reports.

Thank you for contributing to Gix and helping make Git more accessible for everyone.