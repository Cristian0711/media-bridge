#!/usr/bin/env bash
# Enforces the logging severity discipline (see docs/backend-observability-plan.md,
# Phase 5). Run from the backend module root; intended for CI and pre-commit.
#
# Rules:
#   1. ERROR logs must go through logger.Error(ctx, code, msg, err, ...) so every
#      error carries a stable error.code. Direct *.ErrorContext( calls outside the
#      logger package are forbidden.
#   2. os.Exit / *.Fatal are only allowed in cmd/api/main.go (the one bootstrap
#      boundary) — everything else must return errors and shut down gracefully.
set -euo pipefail

fail=0

# Rule 1 — no direct ErrorContext outside shared/logger.
direct_errors=$(grep -rn '\.ErrorContext(' --include='*.go' internal shared cmd \
  | grep -v '_test.go' \
  | grep -v 'shared/logger/' || true)
if [ -n "$direct_errors" ]; then
  echo "✖ ERROR logs must use logger.Error(ctx, code, msg, err, ...) (carries error.code):"
  echo "$direct_errors"
  echo
  fail=1
fi

# Rule 2 — no os.Exit / Fatal outside the main bootstrap.
hard_exits=$(grep -rn 'os\.Exit(\|\.Fatal(\|\.Fatalf(' --include='*.go' internal shared cmd \
  | grep -v '_test.go' \
  | grep -v 'cmd/api/main.go' || true)
if [ -n "$hard_exits" ]; then
  echo "✖ os.Exit / Fatal is only allowed in cmd/api/main.go (return errors instead):"
  echo "$hard_exits"
  echo
  fail=1
fi

if [ "$fail" -eq 0 ]; then
  echo "✓ logging discipline checks passed"
fi
exit "$fail"
