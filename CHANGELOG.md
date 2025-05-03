# Changelog

All notable changes to ObsFind will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Qdrant helper functions for extracting integer and float slices from payload fields
- Comprehensive test suite for all Qdrant payload helper functions
- Publishing checklist with steps for preparing the codebase for release
- Contributing guidelines
- Changelog file

### Fixed
- Test failures in `CachedEmbedder` and `HybridEmbedder` tests
- Import issues in model package

## [1.0.0] - YYYY-MM-DD

### Added
- Initial release of ObsFind
- Background daemon for automatic indexing
- CLI interface for searching and managing the index
- Real-time file watching and indexing
- Support for multiple chunking strategies
- Integration with Ollama for local embedding generation
- Integration with Qdrant for vector storage
- Semantic search with tag and path filtering
- Cross-platform support (macOS, Linux, Windows)
- Automatic startup scripts
- Comprehensive documentation
- Docker support

[Unreleased]: https://github.com/yourusername/obsfind/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/yourusername/obsfind/releases/tag/v1.0.0