# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.1] - 2026-05-26

### Changed

- Require Go 1.25.10 to pick up current Go standard-library security fixes
- Refresh Cobra and QR terminal dependencies for CLI maintenance

### Fixed

- Update whatsmeow for improved WhatsApp protocol stability (#6)

## [1.0.0] - 2026-05-20

### Added

- Request on-demand historical chat pages with `whatsapp backfill`
- Report backfill progress with requested pages, synced message count, and availability of older messages

### Changed

- Require Go 1.25+ and refresh WhatsApp, SQLite, and supporting dependencies
- Wait briefly after history sync completion so late-arriving messages are persisted before commands continue

### Fixed

- Resolve LID senders to phone-number JIDs for quoted replies in group chats (#2)
- Improve WhatsApp auth stability with the latest whatsmeow client updates
- Prevent empty local stores from being treated as completed offline syncs
- Use the current reflect pointer kind when rendering selected output fields

## [0.2.1] - 2026-01-20

### Changed

- Release workflow now extracts version and release notes automatically from CHANGELOG.md

## [0.2.0] - 2026-01-02

### Added

- Automatic sync when data is stale
- GitHub Pages landing site
- Homebrew tap auto-update to release workflow

### Changed

- Wrap terminal animation in IIFE with ES6 conventions
- Update whatsmeow to latest version for auth stability
- Clarify CLI is available as an Agent Skill, not follows the standard

### Fixed

- Increase golangci-lint timeout to 5 minutes

## [0.1.0] - 2025-12-23

### Added

- Initial whatsapp-cli implementation

### Fixed

- Use macos-15-intel instead of deprecated macos-13

[1.0.1]: https://github.com/eddmann/whatsapp-cli/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/eddmann/whatsapp-cli/compare/v0.2.1...v1.0.0
[0.2.1]: https://github.com/eddmann/whatsapp-cli/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/eddmann/whatsapp-cli/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/eddmann/whatsapp-cli/releases/tag/v0.1.0
