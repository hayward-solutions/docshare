# Contributing to DocShare

Thank you for your interest in contributing to DocShare! This document provides guidelines and information for contributors.

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Go 1.24+ (for backend and CLI development)
- Node.js 20+ (for frontend development)
- Git

### Development Setup

1. **Fork the repository**
   ```bash
   # Fork the repository on GitHub, then clone your fork
   git clone https://github.com/your-username/docshare.git
   cd docshare
   ```

2. **Add the upstream remote**
   ```bash
   git remote add upstream https://github.com/docshare/docshare.git
   ```

3. **Start the development environment**
   ```bash
   docker-compose -f docker-compose.dev.yml up -d
   ```

   This builds the backend and frontend from local source code.
   To rebuild after making changes:
   ```bash
   docker-compose -f docker-compose.dev.yml up -d --build
   ```

4. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

## Development Guidelines

### Code Style

#### Backend (Go)
- Follow standard Go formatting (`gofmt`)
- Use `golangci-lint` for linting
- Write meaningful commit messages
- Add tests for new functionality
- Use meaningful variable and function names

#### Frontend (TypeScript/React)
- Follow ESLint configuration
- Use TypeScript for all new code
- Use meaningful component and variable names
- Follow React best practices
- Use shadcn/ui components when possible

### Testing

#### Backend Tests
```bash
cd backend
go test ./...
go test -v -race ./...  # Run with race detection
```

#### Frontend Tests
```bash
cd frontend
npm run lint
npm run build
```

### Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation changes
- `style:` for code style changes (formatting, etc.)
- `refactor:` for code refactoring
- `test:` for adding or updating tests
- `chore:` for maintenance tasks

Examples:
```
feat: add file sharing with groups
fix: resolve authentication token expiration
docs: update API documentation
```

## Pull Request Process

1. **Update your branch**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run tests**
   ```bash
   # Backend tests
   cd backend && go test ./...
   
   # Frontend tests
   cd frontend && npm run lint && npm run build
   ```

3. **Create a pull request**
   - Use a descriptive title
   - Reference any relevant issues
   - Describe your changes
   - Add screenshots if applicable

4. **Code Review**
   - Address review feedback promptly
   - Keep the PR focused and small
   - Respond to all comments

## Reporting Issues

### Bug Reports

When reporting a bug, please include:

- **Description**: Clear description of the issue
- **Steps to Reproduce**: Detailed steps to reproduce the issue
- **Expected Behavior**: What you expected to happen
- **Actual Behavior**: What actually happened
- **Environment**: 
  - OS and version
  - Docker version
  - Browser version (if applicable)
- **Logs**: Relevant error messages or logs

### Feature Requests

When requesting a feature, please include:

- **Use Case**: Why you need this feature
- **Proposed Solution**: How you envision the feature working
- **Alternatives Considered**: Any alternative solutions you've thought about

## Development Workflow

### Backend Development

```bash
cd backend

# Install dependencies
go mod download

# Run the server
go run cmd/server/main.go

# Run tests
go test ./...

# Run with live reload (using air)
air
```

### Frontend Development

```bash
cd frontend

# Install dependencies
npm install

# Run development server
npm run dev

# Run tests
npm test

# Build for production
npm run build
```

### CLI Development

```bash
cd cli

# Build
go build -o docshare .

# Run
./docshare --help

# Static analysis
go vet ./...
```

The CLI is a standalone Go module in `cli/` with its own `go.mod`. It has no shared code with the backend — it communicates with the server purely via the REST API.

**Key directories:**
- `cmd/` — Cobra command definitions (one file per command)
- `internal/api/` — HTTP client and API response types
- `internal/config/` — Config file persistence (`~/.config/docshare/`)
- `internal/output/` — Table formatting and JSON output
- `internal/pathutil/` — Path-to-UUID resolution

## Architecture Guidelines

### Backend (Go)
- Use the existing project structure
- Follow clean architecture principles
- Add new handlers to `internal/handlers/`
- Add new services to `internal/services/`
- Add new models to `internal/models/`

### CLI (Go)
- Add new commands to `cli/cmd/` (one file per command, register in `init()`)
- All commands should support `--json` output via the global `flagJSON` flag
- Use `pathutil.Resolve()` when accepting remote file paths from the user
- Use `requireAuth()` at the start of any command that needs authentication

### Frontend (Next.js)
- Use the App Router
- Add new pages to `src/app/`
- Add new components to `src/components/`
- Use shadcn/ui components when possible
- Follow the existing state management pattern with Zustand

## Security Considerations

- Never commit secrets or API keys
- Use environment variables for configuration
- Follow secure coding practices
- Report security vulnerabilities privately

## Documentation

- Update relevant documentation when making changes
- Add inline comments for complex logic
- Update API documentation for endpoint changes
- Update README.md for user-facing changes

## Release Process

Releases are automated through GitHub Actions:

1. Create a new tag: `git tag v1.0.0`
2. Push the tag: `git push origin v1.0.0`
3. GitHub Actions will build and publish Docker images
4. A GitHub release will be created automatically

## Getting Help

- Check existing issues and documentation
- Ask questions in GitHub Discussions
- Join our community (link to be added)

## Code of Conduct

Please be respectful and inclusive in all interactions. See [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) for details.

---

Thank you for contributing to DocShare! 🚀