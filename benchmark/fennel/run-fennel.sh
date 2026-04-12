#!/usr/bin/env bash
# Runs fennel with fennel-cljlib paths configured
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB="$SCRIPT_DIR/lib"

ASYNC="$LIB/.deps/git/io.gitlab.andreyorst/async.fnl/$(ls "$LIB/.deps/git/io.gitlab.andreyorst/async.fnl")"
ITABLE="$LIB/.deps/git/io.gitlab.andreyorst/itable/$(ls "$LIB/.deps/git/io.gitlab.andreyorst/itable")"
LAZY="$LIB/.deps/git/io.gitlab.andreyorst/lazy-seq/$(ls "$LIB/.deps/git/io.gitlab.andreyorst/lazy-seq")"
READER="$LIB/.deps/git/io.gitlab.andreyorst/reader.fnl/$(ls "$LIB/.deps/git/io.gitlab.andreyorst/reader.fnl")"
UUID="$LIB/.deps/git/io.gitlab.andreyorst/uuid.fnl/$(ls "$LIB/.deps/git/io.gitlab.andreyorst/uuid.fnl")"
REDUCED="$LIB/.deps/git/io.gitlab.andreyorst/reduced.lua/$(ls "$LIB/.deps/git/io.gitlab.andreyorst/reduced.lua")"
LUAINST="$LIB/.deps/git/io.gitlab.andreyorst/lua-inst/$(ls "$LIB/.deps/git/io.gitlab.andreyorst/lua-inst")"
RBTREE="$LIB/.deps/git/io.gitlab.andreyorst/immutableredblacktree.lua/$(ls "$LIB/.deps/git/io.gitlab.andreyorst/immutableredblacktree.lua")"

exec fennel \
  --add-fennel-path "$LIB/src/?.fnl" \
  --add-fennel-path "$LIB/src/?/init.fnl" \
  --add-macro-path "$LIB/src/?.fnlm" \
  --add-macro-path "$LIB/src/?/init.fnlm" \
  --add-fennel-path "$ASYNC/src/?/init.fnl" \
  --add-macro-path "$ASYNC/src/?/init.fnlm" \
  --add-fennel-path "$ITABLE/src/?.fnl" \
  --add-fennel-path "$LAZY/src/?/init.fnl" \
  --add-macro-path "$LAZY/src/?/init.fnlm" \
  --add-fennel-path "$READER/src/?.fnl" \
  --add-fennel-path "$UUID/src/?.fnl" \
  --add-package-path "$REDUCED/src/?.lua" \
  --add-package-path "$LUAINST/src/?.lua" \
  --add-package-path "$RBTREE/src/?.lua" \
  "$@"
