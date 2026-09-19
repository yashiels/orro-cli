# orro-cli: Tuya app-api transport (`orro` account layer)

plan-forge plan, v1, 2026-09-10. repo: yashiels/orro-cli (Go 1.22). author: jarvis.

## 0. Scope

Add the reverse-engineered Tuya **app-api** (SmartLife mobile API) as a second cloud
transport inside orro-cli, beside the existing LAN protocol and IoT OpenAPI clients.
Outcome: `orro` covers the user's whole Tuya account (device list, DP read/write, later
scenes) instead of only the desk. Desk default path stays LAN.

Domain rules: repo AGENTS.md applies (go vet, go test -race, golangci-lint, `make ci`).
Fleet rules: worktree + issue + `type(scope): summary #N` commit + PR. No AI attribution.
Worktree: `/Users/yashiel/Developer/github/yashiels/orro-cli/.worktrees/appapi-transport`,
branch `agent/appapi-transport`, `.worktrees/` in `.git/info/exclude`.

## 1. Ground truth (already extracted from SmartLife 7.11.0 APK, verified)

- App profile: app_id `ekmnwp9f5pnh3trdtpgy`, app_secret in
  `com/thingclips/sample/BuildConfig.java` (7.11.0 jadx tree; matches pypi tuya-mobile
  `smart_life` profile), package `com.tuya.smartlife`, ttid `smartlife`. Constants live
  in `profile.go`, not in this doc.
- Signing cert: tuya's real v3 APK signing block cert, SHA256
  `0F:C3:61:99:9C:C0:C3:5B:A8:AC:A5:7D:AA:55:93:A2:0C:F5:57:27:70:2E:A8:5A:D7:B3:22:89:49:F8:88:FE`
  (CN=www.tuya.com, O=tlife). Extracted from the APK v2/v3 signing block directly.
- Composite signing key: `{package}_{certSHA256}_{embeddedKey}_{appSecret}`;
  embeddedKey `jfg5rs5kkmrj5mxahugvucrsvw43t48x` per pypi tuya-mobile profile (7.10.0
  constants). BMP/cers steganography not needed.
- Sign: whitelist filter (~20 params) -> alphabetical sort -> `k=v` joined `||` ->
  postData value = base64(md5(postData)) with 8/16-char block swap
  (`[0:8][8:16][16:24][24:32] -> [8:16][0:8][24:32][16:24]`) -> HMAC-SHA256 -> hex.
- Body encryption et=3: AES-128-GCM; key = HMAC-SHA256(key=requestId,
  msg=composite+"_"+ecode) -> first 16 hex chars; nonce 12B prepended, 16B tag appended,
  base64.
- Login: `thing.m.user.email.password.login` v1.0, postData
  `{countryCode, email, passwd, ifencrypt, token}`; response carries sid + region.
  First login from a new device may require email OTP.
- Device surface: 133 api names extracted (device_api_names.txt). Core:
  `m.life.device.ext.prop.list` (account devices), `thing.m.device.dp.get`,
  `thing.m.device.dp.publish` v1.0 `{devId, dps}`.
- Host per region: decrypted from APK `thing_domains_v1/regions` blob: e.g. EU
  `https://a1-eu.lifeaiot.com/api.json`, mqtt `mq.mb.tuyaeu.com`. Region picked from
  login response.
- MQTT push password (cmd=2, optional later): mid16(MD5(MD5(composite)+ecode)).
- chKey: DERIVED, not profile-supplied — confirmed against the APK call
  `getChKey(appId)`: `chKey = HMAC-SHA256(key=app_id, msg="{package}_{colonhex(cert)}")
  .hex()[8:16]` = `ec9709a4` for this app (pypi tuya-mobile `channel_key()`, matching
  thekoma's frida capture). Input is app_id + package + cert hash only — no
  version-rotated secret, so 7.10 vs 7.11 rotation risk is nil. Implemented in Go in
  phase 1 with a committed fixture vector.
- embeddedKey/app_key: same source (tuya-mobile profile, 7.10.0). RISK: 7.11.0 may
  rotate it. Mitigation: extract independently from 7.11.0 `assets/fixed_key.bmp` via
  the nalajcie read-keys-from-bmp algorithm BEFORE phase 2; if extraction matches the
  profile value, use it; if extraction fails, fall back to profile value and gate
  phase 2 on a live login probe (single request; `ILLEGAL_CLIENT`/sign error =
  embeddedKey rotated => escalate to frida memory dump of the running app).
  Note: chKey does NOT share this risk (see above — no version-rotated input).

## 2. Deliverables

### Phase 1 — crypto + fixtures (no network)
- `internal/tuya/appapi/profile.go` — constants struct `Profile` (app id/secret/cert
  hex-colon form, embedded key, package, ttid, api versions, region hosts map),
  version-bump = edit one struct.
- `internal/tuya/appapi/crypto.go` — `CompositeKey`, `DeriveEt3Key`, `EncryptEt3GCM`,
  `DecryptEt3GCM`, `PostDataDigestSwap`, `MQTTCmd2Password`. stdlib only
  (crypto/hmac, sha256, md5, aes, cipher.NewGCM).
- `internal/tuya/appapi/sign.go` — whitelist, `CanonicalizeParams`, `Sign`.
- `tools/appapi-fixtures/gen.py` — python fixture generator that imports pypi
  `tuya-mobile` (PurePythonTuyaSigner) and emits JSON vectors with fixed
  requestId/nonce/ecode: composite key, derived et3 key (hex + ascii-form both),
  encrypted postData, swapped digest, canonical strings, final signs, mqtt password.
- `internal/tuya/appapi/testdata/*.json` — committed vectors.
- `crypto_test.go`, `sign_test.go` — table tests over vectors. Both encodings of the
  et3 key tried; whichever matches python becomes the implementation, the other is
  documented as the rejected interpretation.
- Exit: `go test ./internal/tuya/appapi/...` green, `make ci` green.

### Phase 2 — client + auth (network, read-only)
- `internal/tuya/appapi/client.go` — http client: request assembly (url params +
  encrypted body + sign), region host resolution, retries, strict response envelope
  handling (success/errorCode mapping to typed errors).
- **Pre-login host bootstrap (deterministic, runs BEFORE the first credentialed
  login)**: `Login` calls `thing.m.user.region.list` (no session required) with the
  `countryCode` up front and resolves the host from its response; the static region
  map in `profile.go` (all four regions decrypted from the APK) is only the
  fallback when region.list fails. Resolution order: explicit config override >
  region.list response > profile default (EU for this deployment). Mismatch/error
  handling: region.list error code mapped to a typed error; on `REGION_NOT_FOUND`
  the profile default is used and the login attempt proceeds, with the response's
  region correcting the stored session post-login. No login is attempted against a
  guessed region first.
- `internal/tuya/appapi/auth.go` — `Login(countryCode, email, password)`:
  password login; on OTP-required response expose `LoginChallenge`/`CompleteChallenge`
  (two-step, mirroring takealot-cli's handshake UX). The challenge/verify wire
  contract is LOCKED FROM JADX BEFORE CODING: step 0 of phase 2 extracts the exact
  api names, postData/query fields, and the login->challenge->verify transition +
  error codes from the decompiled user api inventory (`pqdbppq.java` + login business
  classes) into committed testdata JSON (same fixtures pipeline as phase 1). Expected
  shape (to be confirmed, not assumed): `thing.m.user.email.code.send` (send OTP),
  `thing.m.user.email.code.login` (complete login with code), challenge state =
  fresh requestId + prelogin token carried from response fields. A MOCKED handshake
  test (httptest server replaying the captured envelope shapes) is REQUIRED green
  before any live login, including the dead-end paths: wrong code, expired token,
  resend.
- Session persisted in `~/.config/orro/session.json` (0600, XDG): sid, region,
  expiry. Writes are atomic (temp file + rename) and guarded by a cross-process
  flock (same pattern as takealot-cli's serialized writes) so concurrent CLI
  invocations never corrupt or race the session. Reuse-or-relogin on 401-ish errors.
- Config layering per repo convention: flags > env (`ORRO_SMARTLIFE_EMAIL`,
  `ORRO_SMARTLIFE_PASSWORD`, op-sa/1Password friendly) > TOML > stored session.
- Exit: `orro login` works live against the user's account (user-approved test);
  read-only device-list call executed via the client package in tests is optional
  and gated on credentials being present.

### Phase 3 — device surface (write path)
- `internal/tuya/appapi/device.go` — `DeviceList`, `DPGet`, `DPPublish`.
- Cobra: `orro devices` (list, --json), `orro dp get <devId>`, `orro dp set <devId>
  <dp> <value>`, `orro logout`, `orro login` (phase 2).
- Write default is ENFORCED: `orro dp set` without `--confirm` prints the exact
  request it would publish and exits 0 without sending. Real writes require
  `--confirm`. Covered by tests: no-send default, send-with-confirm, malformed dp
  rejected before any network call. Desk writes stay on the LAN path regardless.
- Desk integration: `orro status` gains an app-api fallback tier only when LAN and
  IoT cloud both fail. No default behavior change.
- Exit: desk still controlled via LAN; `orro devices` shows the full account; dp set
  verified against a non-desk test device if available.

### Phase 4 (out of scope, noted) — mqtt push, scenes, local key export.

## 3. Risks + mitigations
- et3 key byte encoding ambiguity (ascii hex chars vs decoded nibble bytes) — fixtures
  decide, both tried in phase 1.
- whitelist drift between app versions — whitelist is data (slice), covered by fixture
  round-trip test.
- version pinning: profile struct + `--app-version` override flag; upgrade = edit
  constants; test asserting request defaults match profile.
- MFA/email OTP on first login: two-step challenge flow (phase 2), same UX as
  takealot-cli.
- chKey: derived in Go from app_id/package/cert with fixture vector (risk closed);
  only a live-probe rejection reopens it.
- account safety: dp publish dry-run default + `--confirm`; never auto-send writes.
- no secrets in repo: fixtures carry no real credentials; session file 0600.

## 4. Test strategy
- golden fixtures vs python reference (phase 1, blocking).
- `go test -race ./...` + golangci-lint via make ci.
- live smoke (manual, user-gated): login, devices, dp get; one dp set with --confirm.
- parity: phase 2 optional env-gated test comparing go vs python client canonical
  strings on a real request.

## 5. Rollout
1. issue #N filed for the whole feature (this plan referenced).
2. worktree `agent/appapi-transport` from current main.
3. phases 1-3 as stacked commits (one per phase), squashed per phase on merge;
   PR per phase or single PR after user gate — decided at approval time.
4. `make ci` green before every push.

## 6. Sign-off requested
Reviewer: confirm (a) phase ordering and gates, (b) crypto function set + fixture
approach is sufficient to de-risk the et3/whitelist ambiguities, (c) auth/session
design matches the takealot-cli pattern (env injection + challenge handshake),
(d) write-safety default (dp set --confirm) is right, (e) anything missed that
blocks first login on a fresh device.
