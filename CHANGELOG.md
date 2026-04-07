# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
from v1.0.0 onward. During the v0.x line, breaking changes may land in any release.

## [Unreleased]

## [0.1.0-alpha] - 2026-04-06

### Added
- Provider interface with implementations for Cloudflare, Route53, GoDaddy, and Google Cloud DNS
- `DetectProvider` with NS pattern matching and RDAP fallback
- `PushService` for idempotent record push with post-write verification
- SPF merge algorithm with 10-lookup warning
- Domain Connect discovery, signing, and apply URL generation
- Template engine with sync, resolve, and apply
- `entree` CLI with `detect`, `apply`, `verify`, `spf-merge`, `dc-discover`, and `templates` commands
