# Saws Window UI Design (Tauri + React)

## Goal

Provide a cross-platform desktop window app for Linux, macOS, and Windows that makes `saws` easier for non-terminal users, while preserving the existing CLI as the single source of truth for AWS auth, discovery, and credential management.

## Decisions Locked

- Platform: Linux, macOS, Windows
- App type: Standard window app (no tray-first requirement in v1)
- Tech stack: Tauri + React
- Repository layout: New folder in this repo (`ui/`)
- Runtime strategy: Bundle the `saws` binary with the app
- Auth UX: External browser flow for AWS device authorization

## Non-Goals (v1)

- No tray/menu-bar mode
- No background daemon or persistent agent
- No direct AWS SDK calls from UI layer
- No alternate credential storage model beyond existing `saws` behavior

## Architecture

### High-level

1. React frontend renders app screens and user interactions.
2. Tauri backend exposes a thin set of commands to the frontend.
3. Tauri commands execute bundled `saws` subprocesses and return normalized JSON.
4. `saws` continues to own:
   - OIDC device auth flow
   - account/role discovery
   - writing AWS config/credentials
   - token cache handling

### Why this architecture

- Minimizes security drift by reusing audited CLI logic.
- Reduces implementation risk and time-to-first-release.
- Keeps one behavior contract for both CLI and UI users.

## UX Flows

## 1) Connect (first run)

- User opens app, sees "Connect to AWS SSO."
- App requests setup/discovery through Tauri command.
- During auth, app displays clear state:
  - "Opening browser for sign-in..."
  - "Waiting for approval..."
- User completes auth in external browser.
- App proceeds to discovery and transitions to profile selection.

## 2) Profile selection and switching

- Searchable profile list grouped by account/role (using data from `saws`).
- User selects profile and clicks "Use this profile."
- App triggers credential refresh/export-backed flow and confirms success.

## 3) Session status

- Show active profile.
- Show credential/token expiration status.
- Provide clear action: "Refresh credentials."

## 4) Settings and diagnostics

- Show app version + bundled `saws` version.
- Show paths being used (sanitized).
- Include a diagnostics export action with no secret material.

## Data Contract

The UI requires structured outputs from CLI-facing operations. If current flags do not expose sufficient machine-readable output, add focused JSON-capable modes in `saws` later (implementation-phase task).

The Tauri layer normalizes subprocess output into a stable UI contract:

- `status`: success | recoverable_error | fatal_error
- `code`: typed error code (e.g. `auth_expired`, `network_error`)
- `message`: user-display message
- `payload`: operation-specific fields (profiles, expiry timestamps, etc.)

## Error Handling Model

Map raw failures to user-friendly categories:

- Authentication required/expired
- Network/service error
- No accounts or roles available
- Local path/permission/write issue
- Unexpected subprocess failure

Each category has one primary call to action:

- Retry
- Re-authenticate
- Open settings/troubleshooting

## Security Requirements

- UI never persists AWS secrets.
- All secret-like values are masked in UI and logs.
- Diagnostics output must exclude access tokens and credential values.
- Subprocess invocation must avoid shell interpolation risks (direct args only).

## Packaging and Distribution

- Build installers per platform via Tauri pipeline.
- Ship architecture-matched `saws` binary in app resources.
- On startup, verify bundled binary is present and executable.
- Display actionable error if binary is missing/corrupt.

## Testing Strategy

## Contract tests

- Validate structured CLI outputs consumed by Tauri commands.
- Include expected error-code mapping cases.

## UI integration tests

- First-time connect + browser auth waiting state
- Profile selection and switch success path
- Expired auth recovery path

## Manual smoke matrix

- Linux, macOS, Windows:
  - startup
  - connect
  - switch profile
  - refresh session

## Proposed Folder Shape

```text
ui/
  src/                  # React app
  src-tauri/            # Tauri backend and config
  tests/                # UI/integration tests
  package.json
  tauri.conf.json
```

## Risks and Mitigations

- Risk: CLI/UI contract churn
  - Mitigation: version and stabilize Tauri command payloads early.
- Risk: cross-platform packaging friction
  - Mitigation: CI matrix for installer build smoke tests.
- Risk: auth UX confusion during browser handoff
  - Mitigation: explicit in-app progress states and retry guidance.

## Success Criteria (v1)

- New user can install app and complete first AWS connect without terminal usage.
- User can choose a profile and successfully use credentials in standard AWS tooling.
- Session status and refresh actions are understandable and reliable.
- App works on Linux/macOS/Windows with bundled `saws`.

## Next Step

Create an implementation plan from this design (task breakdown, milestones, and acceptance tests) before writing code.
