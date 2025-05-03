# Contributing to ObsFind

Thank you for considering contributing to ObsFind! This document provides guidelines and instructions for contributing to the project.

## Code of Conduct

Please be respectful and considerate of others when contributing to ObsFind. We expect all contributors to adhere to the following principles:

- Be respectful and inclusive
- Be patient and welcoming
- Be thoughtful in communication
- Focus on what is best for the community
- Show empathy towards other community members

## How Can I Contribute?

### Reporting Bugs

- **Check Existing Issues** to see if the bug has already been reported
- **Use the Bug Report Template** if available
- **Include Detailed Information**:
  - Steps to reproduce the issue
  - Expected behavior
  - Actual behavior
  - Version information (Go version, ObsFind version, OS)
  - Logs or error messages
  - Screenshots if applicable

### Suggesting Features

- **Check Existing Issues** to see if the feature has already been suggested
- **Use the Feature Request Template** if available
- **Be Specific** about the feature and why it would be valuable
- **Consider the Scope** of the feature and how it fits into the project

### Pull Requests

1. **Fork the Repository**
2. **Create a Branch** with a descriptive name
3. **Make Your Changes**:
   - Follow the coding style and guidelines
   - Write tests for your changes
   - Update documentation as necessary
4. **Run the Tests** to ensure your changes don't break existing functionality
5. **Submit a Pull Request**:
   - Describe the changes you've made
   - Reference any related issues
   - Follow the PR template if available

## Development Setup

### Prerequisites

- Go 1.21 or later
- Ollama (for local embedding)
- Qdrant (for vector storage)

### Setting Up Development Environment

1. **Clone the Repository**
   ```bash
   git clone https://github.com/yourusername/obsfind.git
   cd obsfind
   ```

2. **Install Dependencies**
   ```bash
   go mod download
   ```

3. **Run Tests**
   ```bash
   go test -v ./...
   ```

4. **Build the Project**
   ```bash
   make build
   ```

### Code Style and Guidelines

- Follow [Effective Go](https://golang.org/doc/effective_go) principles
- Use [gofmt](https://golang.org/cmd/gofmt/) to format code
- Add comments and documentation for public functions and packages
- Write unit tests for new functionality

## Project Structure

```
obsfind/
├── cmd/                # Command-line applications
│   ├── cli/            # CLI client
│   └── daemon/         # Background service
├── internal/           # Internal packages
│   ├── api/            # API server
│   ├── config/         # Configuration
│   ├── embedding/      # Embedding generation
│   ├── filewatcher/    # File monitoring
│   ├── indexer/        # Document indexing
│   ├── markdown/       # Markdown parsing
│   ├── model/          # Data models
│   ├── qdrant/         # Qdrant client
│   └── search/         # Search implementation
├── pkg/                # Public API packages
│   ├── cli/            # CLI library
│   └── types/          # Shared types
└── scripts/            # Utility scripts
```

## Testing

- Write unit tests for all new functionality
- Ensure tests are isolated and don't depend on external services when possible
- Mock external dependencies (Qdrant, embedding models)
- Aim for >80% code coverage

## Documentation

- Document all public functions, types, and packages
- Update README.md and other documentation as necessary
- If adding CLI commands, document them in usage examples

## Release Process

1. Update version in relevant files
2. Update CHANGELOG.md
3. Tag the release
4. Create a GitHub release with release notes

## Questions?

If you have any questions about contributing, please open an issue for discussion.

Thank you for contributing to ObsFind!