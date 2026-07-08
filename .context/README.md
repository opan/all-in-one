# Project Context Documentation

This directory contains implementation plans, progress trackers, ADRs (Architecture Decision Records), and other technical documentation for features in the all-in-one project.

## 📁 Files

### Chat Feature
- **[CHAT_IMPLEMENTATION_PLAN.md](CHAT_IMPLEMENTATION_PLAN.md)** - Original implementation plan for the chat feature
- **[CHAT_PROGRESS.md](CHAT_PROGRESS.md)** - Progress tracker for chat feature development (~97% complete)
- **[CHAT_SCHEMA_UPDATE.md](CHAT_SCHEMA_UPDATE.md)** - Database schema changes and design rationale
- **[WEBSOCKET_IMPLEMENTATION.md](WEBSOCKET_IMPLEMENTATION.md)** - WebSocket implementation summary with issues resolved

### Listing Feature
- **[LISTING_UI_IMPLEMENTATION.md](LISTING_UI_IMPLEMENTATION.md)** - UI implementation guide for listing feature
- **[JSONFORMS_SCHEMA_GUIDE.md](JSONFORMS_SCHEMA_GUIDE.md)** - Guide for creating form schemas using JSONForms

### Access Management (RBAC)
- **[RBAC_IMPLEMENTATION_PLAN.md](RBAC_IMPLEMENTATION_PLAN.md)** - Approved plan for the admin-only Access Management / feature-authorization system
- **[RBAC_PROGRESS.md](RBAC_PROGRESS.md)** - Progress tracker for RBAC development (complete)

### Admin & User Management
- **[USER_ADMIN_MANAGEMENT_IMPLEMENTATION_PLAN.md](USER_ADMIN_MANAGEMENT_IMPLEMENTATION_PLAN.md)** - Plan for the dedicated Admin area + user management (edit email, block/unblock login)
- **[USER_ADMIN_MANAGEMENT_PROGRESS.md](USER_ADMIN_MANAGEMENT_PROGRESS.md)** - Progress tracker (complete)

### Other
- **[SWAGGER_INTEGRATION.md](SWAGGER_INTEGRATION.md)** - Swagger/OpenAPI integration documentation

## 📝 Purpose

These files serve as:
- **Progress Trackers** - Track feature completion status
- **ADRs** - Document architectural and design decisions
- **Implementation Guides** - Help developers understand implementation details
- **Context for AI** - Provide Copilot/AI assistants with project context

## 🔄 Usage

When starting a new feature or chat session:
1. Review relevant documentation in this directory
2. Create new progress tracker if needed (e.g., `FEATURE_PROGRESS.md`)
3. Document important decisions in ADR format
4. Move completed documentation here for archival

## 📋 File Naming Convention

- `*_IMPLEMENTATION_PLAN.md` - Initial planning documents
- `*_PROGRESS.md` - Active progress trackers
- `*_SCHEMA_UPDATE.md` - Database schema changes
- `*_IMPLEMENTATION.md` - Implementation summaries/ADRs
- `*_GUIDE.md` - How-to guides and references

## 🎯 Maintenance

- Update progress files as features are completed
- Create new ADRs when making significant architectural decisions
- Archive completed feature docs here
- Keep this README updated with new files
