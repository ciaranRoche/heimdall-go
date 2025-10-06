# Contributing to Heimdall Go

Thank you for your interest in contributing to Heimdall Go! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for all contributors.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/heimdall-go.git
   cd heimdall-go
   ```
3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/ciaranRoche/heimdall-go.git
   ```

## Development Setup

### Prerequisites

- Go 1.21 or later
- Make (optional, but recommended)
- Docker (for integration tests)

### Install Dependencies

```bash
go mod download
```

### Development Tools

Development tools like `golangci-lint` are managed by [bingo](https://github.com/bwplotka/bingo) and will be automatically installed at the correct version when you run commands like `make lint`. No manual installation needed!

Tool versions are pinned in `.bingo/` directory to ensure consistency across the team.

## Making Changes

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
```

Use descriptive branch names:
- `feature/` for new features
- `fix/` for bug fixes
- `docs/` for documentation changes
- `refactor/` for code refactoring

### 2. Make Your Changes

- Write clear, concise code
- Follow Go best practices and idioms
- Add tests for new functionality
- Update documentation as needed

### 3. Format and Lint

```bash
make fmt
make vet
make lint
```

### 4. Run Tests

```bash
make test
```

For coverage report:
```bash
make test-coverage
```

### 5. Commit Your Changes

Write clear, descriptive commit messages:

```bash
git commit -m "feat: add support for custom message serialization"
```

Follow conventional commit format:
- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation changes
- `test:` for test additions/changes
- `refactor:` for code refactoring
- `chore:` for maintenance tasks

### 6. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub.

## Pull Request Guidelines

### PR Description

Include:
- **Summary**: Brief description of changes
- **Motivation**: Why is this change needed?
- **Implementation**: How does it work?
- **Testing**: How was it tested?
- **Breaking Changes**: Any breaking changes?

### PR Checklist

- [ ] Code follows project style guidelines
- [ ] Tests added/updated and passing
- [ ] Documentation updated
- [ ] Commit messages are clear and descriptive
- [ ] No unnecessary dependencies added
- [ ] PR title follows conventional commit format

## Code Style

### General Guidelines

- Use `gofmt` for formatting (automated via `make fmt`)
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Keep functions small and focused
- Use meaningful variable and function names
- Add comments for exported types and functions

### Package Organization

```
heimdall-go/
├── heimdall.go           # Public API
├── config.go             # Configuration
├── provider/             # Provider interface and implementations
│   ├── provider.go       # Interface and registry
│   ├── kafka/           # Kafka provider
│   └── rabbitmq/        # RabbitMQ provider
├── routing/             # Routing engine (future)
├── observability/       # Observability (future)
├── internal/            # Private implementation details
└── examples/            # Example applications
```

### Testing Guidelines

1. **Unit Tests**
   - Test file: `*_test.go`
   - Test function: `TestFunctionName`
   - Use table-driven tests when appropriate

2. **Test Coverage**
   - Aim for >80% coverage
   - Focus on critical paths
   - Don't test trivial code

3. **Integration Tests**
   - Use testcontainers for real services
   - Tag with `// +build integration`
   - Run separately from unit tests

### Example Test

```go
func TestPublish(t *testing.T) {
    tests := []struct {
        name    string
        topic   string
        data    []byte
        wantErr bool
    }{
        {
            name:    "valid message",
            topic:   "test.topic",
            data:    []byte("test"),
            wantErr: false,
        },
        {
            name:    "empty topic",
            topic:   "",
            data:    []byte("test"),
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Adding a New Provider

To add a new messaging provider:

1. **Create provider package**: `provider/yourprovider/`
2. **Implement Provider interface**:
   ```go
   type Provider interface {
       Publish(ctx context.Context, topic string, data []byte, headers map[string]interface{}, correlationID string) error
       Subscribe(ctx context.Context, topic string, handler MessageHandler) error
       HealthCheck(ctx context.Context) error
       Close() error
   }
   ```
3. **Register in init()**:
   ```go
   func init() {
       provider.Register("yourprovider", NewProvider)
   }
   ```
4. **Add tests**: `provider/yourprovider/yourprovider_test.go`
5. **Add example**: `examples/yourprovider/`
6. **Update documentation**: Add provider to README.md

## Managing Development Tools with Bingo

This project uses [bingo](https://github.com/bwplotka/bingo) to manage development tool versions. Tools are automatically installed at the correct version when needed.

### Adding a New Tool

To add a new development tool:

```bash
# Add latest version
bingo get github.com/tool/name

# Add specific version
bingo get github.com/tool/name@v1.2.3
```

This creates:
- `.bingo/tool.mod` - Go module for the tool
- `.bingo/tool.sum` - Checksums for dependencies
- Updates `.bingo/Variables.mk` - Makefile variables

### Using Tools in Makefile

Tools are available as make targets:

```makefile
# Add to target dependencies
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run ./...
```

The tool will be automatically installed at the pinned version if not present.

### Updating Tool Versions

```bash
# Update to latest version
bingo get github.com/tool/name@latest

# Update to specific version
bingo get github.com/tool/name@v2.0.0
```

Always commit `.bingo/` changes so the team uses the same tool versions.

## Documentation

- Keep README.md up to date
- Add godoc comments for exported types/functions
- Update examples when adding features
- Create docs/ files for complex features

## Release Process

Releases are managed by project maintainers:

1. Update CHANGELOG.md
2. Tag release: `git tag v0.1.0`
3. Push tag: `git push origin v0.1.0`
4. GitHub Actions will create the release

## Getting Help

- **Issues**: Open an issue on GitHub
- **Discussions**: Use GitHub Discussions for questions
- **Email**: Contact maintainers directly for sensitive issues

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

## Recognition

Contributors will be recognized in:
- GitHub contributors list
- CHANGELOG.md for significant contributions
- Project README.md (optional)

Thank you for contributing to Heimdall Go! 🎉
