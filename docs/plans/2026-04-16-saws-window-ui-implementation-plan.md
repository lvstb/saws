# Saws Window UI Implementation Plan

**Goal:** Build a cross-platform Tauri + React window app in `ui/` that lets non-terminal users authenticate with AWS SSO, choose a profile, and view session status by invoking the bundled `saws` binary.
**Architecture:** Keep `saws` as the source of truth for auth, discovery, config writes, and token cache behavior. Add small machine-readable CLI contracts in the Go app, then build a thin Tauri command layer that invokes the bundled binary and returns normalized data to a React UI.
**Tech Stack:** Go CLI (`main.go`, `internal/*`), Tauri, React, TypeScript, npm, GitHub Actions

---

### Task 1: Add machine-readable CLI contract for the UI

**Files:**
- Modify: `main.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/sso_cache.go`
- Modify: `internal/profile/profile.go`
- Create: `internal/appjson/appjson.go`
- Create: `internal/appjson/appjson_test.go`
- Create: `internal/appstatus/appstatus.go`
- Create: `internal/appstatus/appstatus_test.go`
- Modify: `README.md`

**Step 1: Write the failing tests**

Add tests that define the JSON payloads the UI needs:
- `internal/appjson/appjson_test.go`
  - `TestMarshalProfilesResponse`
  - `TestMarshalSessionStatusResponse`
  - `TestMarshalErrorResponse`
- `internal/appstatus/appstatus_test.go`
  - `TestLoadSessionStatus_WithSavedProfilesAndValidCache`
  - `TestLoadSessionStatus_NoProfiles`
  - `TestLoadSessionStatus_ExpiredCache`

Expected payload shapes:
- profiles list: profile name, account name, account ID, role name, region
- session status: active/selected profile if known, cache expiry, hasProfiles boolean
- typed errors: `status`, `code`, `message`, optional `payload`

**Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/appjson ./internal/appstatus
```

Expected: FAIL because packages and functions do not exist yet.

**Step 3: Write minimal implementation**

Create:
- `internal/appjson/appjson.go`
  - response structs for success/error payloads
  - helper functions for JSON encoding
- `internal/appstatus/appstatus.go`
  - read saved profiles from `internal/config`
  - inspect token cache via `internal/config/sso_cache.go`
  - return UI-facing status model

Modify `main.go` to add minimal UI-oriented flags:
- `--profiles-json`
- `--status-json`
- `--login-json --profile <name>` or equivalent focused command shape

Keep output strictly JSON on stdout for these modes.

**Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/appjson ./internal/appstatus
```

Expected: PASS

**Step 5: Verify CLI behavior manually**

Run:
```bash
go test ./...
go run . --profiles-json
go run . --status-json
```

Expected:
- tests pass
- JSON commands print valid JSON without TUI text mixed in

**Step 6: Commit**

```bash
git add main.go internal/appjson internal/appstatus internal/config/config.go internal/config/sso_cache.go internal/profile/profile.go README.md
git commit -m "feat: add JSON interface for desktop app"
```

### Task 2: Scaffold the Tauri + React app in `ui/`

**Files:**
- Create: `ui/package.json`
- Create: `ui/package-lock.json`
- Create: `ui/tsconfig.json`
- Create: `ui/vite.config.ts`
- Create: `ui/index.html`
- Create: `ui/src/main.tsx`
- Create: `ui/src/App.tsx`
- Create: `ui/src/styles.css`
- Create: `ui/src-tauri/Cargo.toml`
- Create: `ui/src-tauri/build.rs`
- Create: `ui/src-tauri/tauri.conf.json`
- Create: `ui/src-tauri/src/main.rs`
- Create: `ui/src-tauri/src/lib.rs`
- Modify: `.gitignore`

**Step 1: Write the failing tests**

Create basic frontend smoke tests:
- `ui/src/App.test.tsx`
  - renders app shell title
  - renders connect state placeholder

Create Tauri-side smoke tests if practical:
- `ui/src-tauri/src/lib.rs`
  - unit test for app command registration or command result normalization helper

**Step 2: Run tests to verify they fail**

Run:
```bash
cd ui && npm test
```

Expected: FAIL because the app is not scaffolded yet.

**Step 3: Write minimal implementation**

Scaffold a standard Tauri + React app:
- Vite + React + TypeScript frontend
- Tauri Rust shell backend
- top-level window layout with placeholder sections:
  - Connect
  - Profiles
  - Session
  - Settings

Update `.gitignore` for:
- `ui/node_modules/`
- `ui/dist/`
- `ui/src-tauri/target/`

**Step 4: Run tests to verify they pass**

Run:
```bash
cd ui && npm test
```

Expected: PASS

**Step 5: Verify app boots**

Run:
```bash
cd ui && npm install
cd ui && npm run tauri dev
```

Expected:
- Tauri window opens
- placeholder shell renders without crashing

**Step 6: Commit**

```bash
git add ui .gitignore
git commit -m "feat: scaffold Tauri React desktop app"
```

### Task 3: Add bundled-binary execution from Tauri

**Files:**
- Modify: `ui/src-tauri/tauri.conf.json`
- Modify: `ui/src-tauri/Cargo.toml`
- Modify: `ui/src-tauri/src/lib.rs`
- Create: `ui/src-tauri/src/saws.rs`
- Create: `ui/src/lib/api.ts`
- Create: `ui/src/lib/types.ts`
- Create: `ui/src/lib/errors.ts`
- Create: `ui/src/lib/api.test.ts`

**Step 1: Write the failing tests**

Create tests for the Tauri command adapter:
- `ui/src/lib/api.test.ts`
  - maps `profiles-json` output to frontend types
  - maps status-json output to frontend types
  - maps error payloads to user-friendly codes

If Tauri-side Rust tests are used, add:
- test for binary-path resolution
- test for direct-arg subprocess invocation (no shell wrapper)

**Step 2: Run tests to verify they fail**

Run:
```bash
cd ui && npm test
```

Expected: FAIL because the API layer and Tauri commands do not exist yet.

**Step 3: Write minimal implementation**

Implement:
- `ui/src-tauri/src/saws.rs`
  - resolve bundled `saws` binary
  - execute direct subprocess calls
  - capture stdout/stderr
  - parse JSON output
- `ui/src-tauri/src/lib.rs`
  - expose Tauri commands such as:
    - `get_profiles`
    - `get_status`
    - `login_with_profile`
    - `start_configure`
- `ui/src/lib/api.ts`
  - typed invoke wrappers
- `ui/src/lib/errors.ts`
  - normalize CLI and Tauri failures into UI-friendly categories

Configure bundled resources in `ui/src-tauri/tauri.conf.json` for OS-specific `saws` binaries.

**Step 4: Run tests to verify they pass**

Run:
```bash
cd ui && npm test
```

Expected: PASS

**Step 5: Verify bundled execution path**

Run:
```bash
cd ui && npm run tauri dev
```

Manually verify:
- app can call a harmless status command
- missing binary state yields clear error

**Step 6: Commit**

```bash
git add ui/src-tauri ui/src/lib
git commit -m "feat: invoke bundled saws from desktop app"
```

### Task 4: Build the Connect flow with external browser auth

**Files:**
- Modify: `ui/src/App.tsx`
- Create: `ui/src/features/connect/ConnectView.tsx`
- Create: `ui/src/features/connect/ConnectView.test.tsx`
- Create: `ui/src/features/connect/useConnectFlow.ts`
- Modify: `main.go`
- Modify: `internal/auth/auth.go`
- Modify: `internal/auth/auth_test.go`

**Step 1: Write the failing tests**

Frontend tests:
- render connect CTA when no profiles/session exist
- show progress states:
  - opening browser
  - waiting for approval
  - discovery complete
- show retry button on recoverable error

Go tests:
- `internal/auth/auth_test.go`
  - JSON/UI mode does not emit human-formatted TUI output
  - status events needed by UI are surfaced predictably

**Step 2: Run tests to verify they fail**

Run:
```bash
cd ui && npm test
go test ./internal/auth
```

Expected: FAIL for missing UI flow and missing auth status plumbing.

**Step 3: Write minimal implementation**

Add a UI-safe auth/discovery command path that:
- opens external browser as today
- emits machine-readable progress states
- returns success/failure cleanly to Tauri

Implement frontend connect screen and hook:
- button to start connect flow
- progress panel
- success transition into profiles/session view

**Step 4: Run tests to verify they pass**

Run:
```bash
cd ui && npm test
go test ./internal/auth
```

Expected: PASS

**Step 5: Verify flow manually**

Run:
```bash
cd ui && npm run tauri dev
```

Manual checks:
- click Connect
- browser opens externally
- app shows waiting state
- cancelled/failed login shows retry path

**Step 6: Commit**

```bash
git add ui/src/features/connect main.go internal/auth
git commit -m "feat: add desktop connect flow"
```

### Task 5: Build profile picker and session status screens

**Files:**
- Modify: `ui/src/App.tsx`
- Create: `ui/src/features/profiles/ProfileList.tsx`
- Create: `ui/src/features/profiles/ProfileList.test.tsx`
- Create: `ui/src/features/session/SessionPanel.tsx`
- Create: `ui/src/features/session/SessionPanel.test.tsx`
- Create: `ui/src/features/settings/SettingsPanel.tsx`
- Modify: `ui/src/styles.css`

**Step 1: Write the failing tests**

Frontend tests:
- profiles render grouped/searchable
- clicking “Use profile” triggers API call
- session panel shows expiry state
- settings panel shows app + binary version data

**Step 2: Run tests to verify they fail**

Run:
```bash
cd ui && npm test
```

Expected: FAIL because screens do not exist yet.

**Step 3: Write minimal implementation**

Implement:
- profile list UI
- use-profile action
- session status panel
- simple settings panel with diagnostics info

Keep v1 lean:
- no tray UI
- no background polling daemon
- no advanced profile editing

**Step 4: Run tests to verify they pass**

Run:
```bash
cd ui && npm test
```

Expected: PASS

**Step 5: Verify the complete happy path manually**

Run:
```bash
cd ui && npm run tauri dev
```

Manual checks:
- load app
- see connect or profile state correctly
- select profile
- refresh session
- read expiry info

**Step 6: Commit**

```bash
git add ui/src
git commit -m "feat: add profile and session views"
```

### Task 6: Package bundled binaries and CI smoke coverage

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml`
- Modify: `.goreleaser.yaml`
- Create: `ui/scripts/verify-bundled-saws.mjs`
- Modify: `README.md`
- Modify: `docs/plans/2026-04-16-saws-window-ui-design.md`

**Step 1: Write the failing checks**

Add CI jobs/checks that expect:
- frontend dependencies install
- frontend tests pass
- Tauri app builds in CI smoke mode
- bundled binary verification script passes

**Step 2: Run checks to verify they fail**

Run:
```bash
cd ui && npm ci
cd ui && npm test
node ui/scripts/verify-bundled-saws.mjs
```

Expected: initial failures until packaging paths and scripts exist.

**Step 3: Write minimal implementation**

Update CI/release workflow to include:
- Node setup
- frontend tests
- Tauri build smoke step
- bundled-resource validation

Document desktop app usage in `README.md`.

**Step 4: Run checks to verify they pass**

Run:
```bash
go test ./...
cd ui && npm test
node ui/scripts/verify-bundled-saws.mjs
```

Expected: PASS

**Step 5: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/release.yml .goreleaser.yaml ui/scripts README.md docs/plans/2026-04-16-saws-window-ui-design.md
git commit -m "ci: add desktop app build and packaging checks"
```

## Final Verification Checklist

Run from repo root:

```bash
go test ./...
cd ui && npm test
cd ui && npm run tauri build
```

Expected:
- Go tests pass
- UI tests pass
- desktop app builds successfully

## Suggested Milestones

1. CLI JSON contract stable
2. Tauri shell boots
3. Bundled binary execution works
4. Connect flow works end-to-end
5. Profile/session screens work
6. CI/release smoke coverage added

## Notes

- Keep every subprocess invocation direct; do not use shell evaluation.
- Prefer very small TDD loops for both Go and UI code.
- Do not introduce background refresh, tray UX, or direct AWS SDK calls in the UI during v1.
