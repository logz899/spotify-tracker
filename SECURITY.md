# Security Policy

## Reporting a Vulnerability

Please report vulnerabilities privately through GitHub Security Advisories:
open this repository's **Security** tab and choose **Report a vulnerability**.
Do not open a public issue, pull request, or discussion for a suspected
vulnerability.

Include the affected component (API, sync job, frontend, or deployment
definitions), reproduction steps, and the impact you observed. Please do not
include real Spotify tokens, passwords, or other personal data in the report.

## Supported Versions

Only the latest commit on `main` is supported.

## Scope

In scope: the code and deployment definitions in this repository. Out of scope:
Spotify, Google Cloud, Neon, and GitHub themselves, and denial-of-service tests
against the hosted demo, which runs on free-tier quotas.

## Project Safeguards

- Secrets live only in Google Secret Manager; none are stored in this
  repository. CI scans source and the built frontend with Gitleaks.
- Spotify access and refresh tokens are encrypted at rest (AES-256-GCM).
- OAuth state and pending account-link keys are random, hashed at rest,
  expire after ten minutes, and can be used once.
- Production database connections require TLS, and CORS allows only the
  published frontend origin.
- Dependencies are checked with `govulncheck` and `npm audit` in CI, and
  GitHub Actions are pinned to commit SHAs.

## Known Limitations

- The frontend audit covers runtime dependencies only. Create React App's
  build and test toolchain (`react-scripts`, a devDependency) has advisories
  with no non-breaking fix; that code runs only at build time and is not
  shipped to browsers. Migrating the frontend build to Vite will remove it.
