# Contributing to pbuf-registry

Thank you for your interest in contributing to pbuf-registry! This document provides guidelines and recommendations for contributing to the project.

## How to Contribute

### Reporting Issues

We use GitHub Issues to track bugs, feature requests, and questions. When creating an issue, please use the appropriate template and provide as much detail as possible.

#### Issue Types and Labels

We recommend using the following labels to organize issues:

**Type Labels:**
- `bug` - Something isn't working correctly
- `feature` - New feature or enhancement request
- `documentation` - Documentation improvements
- `question` - General questions about usage
- `performance` - Performance-related issues

**Priority Labels:**
- `priority: high` - Critical issues that need immediate attention
- `priority: medium` - Important but not urgent
- `priority: low` - Nice to have improvements

**Status Labels:**
- `help wanted` - Issues where community contributions are welcome
- `good first issue` - Good for newcomers to the project
- `in progress` - Someone is actively working on this
- `needs discussion` - Requires further discussion before implementation

### Issue Templates

#### Bug Report Template

```markdown
**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Configure with '...'
2. Run command '...'
3. See error

**Expected behavior**
What you expected to happen.

**Actual behavior**
What actually happened.

**Environment:**
- pbuf-registry version: [e.g., v0.4.1]
- pbuf version: [e.g., v0.3.0]
- OS: [e.g., Ubuntu 22.04, macOS 14]
- Deployment method: [Docker Compose / Helm / Binary]

**Logs**
```
Paste relevant logs here
```

**Additional context**
Any other context about the problem.
```

#### Feature Request Template

```markdown
**Is your feature request related to a problem? Please describe.**
A clear and concise description of what the problem is. Ex. I'm always frustrated when [...]

**Describe the solution you'd like**
A clear and concise description of what you want to happen.

**Describe alternatives you've considered**
A clear and concise description of any alternative solutions or features you've considered.

**Use case**
Describe the use case that would benefit from this feature.

**Additional context**
Add any other context or screenshots about the feature request here.
```

#### Question Template

```markdown
**Your Question**
Clearly state your question here.

**What you've tried**
Describe what you've already attempted or researched.

**Context**
Provide any relevant context about your setup or use case.
```

---

## Pull Request Process

1. **Fork and Branch**
   - Fork the repository
   - Create a feature branch: `git checkout -b feature/your-feature-name`

2. **Make Changes**
   - Write clear, concise commit messages
   - Follow the existing code style
   - Add tests for new functionality
   - Update documentation as needed

3. **Test Your Changes**
   ```shell
   make test
   make build
   ```

4. **Submit PR**
   - Push to your fork
   - Create a Pull Request with a clear description
   - Reference any related issues
   - Wait for review and address feedback

5. **PR Review**
   - Maintainers will review your PR
   - Address any requested changes
   - Once approved, a maintainer will merge

## Code Style

- Follow standard Go conventions (`gofmt`, `golint`)
- Write clear comments for public APIs
- Keep functions focused and testable
- Use meaningful variable and function names

## Testing

- Write unit tests for new functionality
- Ensure existing tests pass: `make test`
- Add integration tests for API changes
- Test with Docker Compose setup locally

## Documentation

- Update README.md if you change user-facing features
- Add KDoc comments for public APIs
- Update swagger definitions if you modify the API
- Include examples for new features

## Community

- Be respectful and constructive in discussions
- Help others in issues and discussions
- Share your use cases and feedback
- Contribute to documentation improvements

## Questions?

If you have questions about contributing, feel free to:
- Open an issue with the `question` label
- Start a discussion in GitHub Discussions
- Check existing documentation and examples

Thank you for contributing to pbuf-registry! 🎉
