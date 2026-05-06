#!/usr/bin/env bash
set -euo pipefail

SPDX="SPDX-License-Identifier: MPL-2.0"
MPL_NOTICE_1="This Source Code Form is subject to the terms of the Mozilla Public"
MPL_NOTICE_2="License, v. 2.0. If a copy of the MPL was not distributed with this"
MPL_NOTICE_3="file, You can obtain one at https://mozilla.org/MPL/2.0/."

add_c_style_header() {
  local file="$1"

  grep -q "$MPL_NOTICE_1" "$file" && return 0

  local tmp
  tmp="$(mktemp)"

  {
    echo "// $SPDX"
    echo
    echo "/*"
    echo " * $MPL_NOTICE_1"
    echo " * $MPL_NOTICE_2"
    echo " * $MPL_NOTICE_3"
    echo " */"
    echo
    cat "$file"
  } > "$tmp"

  mv "$tmp" "$file"
}

add_line_comment_header() {
  local file="$1"
  local comment="$2"

  grep -q "$MPL_NOTICE_1" "$file" && return 0

  local tmp
  tmp="$(mktemp)"

  if head -n 1 "$file" | grep -q '^#!'; then
    {
      head -n 1 "$file"
      echo "${comment} $SPDX"
      echo "${comment}"
      echo "${comment} $MPL_NOTICE_1"
      echo "${comment} $MPL_NOTICE_2"
      echo "${comment} $MPL_NOTICE_3"
      tail -n +2 "$file"
    } > "$tmp"
  else
    {
      echo "${comment} $SPDX"
      echo "${comment}"
      echo "${comment} $MPL_NOTICE_1"
      echo "${comment} $MPL_NOTICE_2"
      echo "${comment} $MPL_NOTICE_3"
      echo
      cat "$file"
    } > "$tmp"
  fi

  mv "$tmp" "$file"
}

git ls-files -z --cached --others --exclude-standard \
  | while IFS= read -r -d '' file; do
      case "$file" in
        vendor/*|node_modules/*|dist/*|build/*|.git/*)
          continue
          ;;

        *.go|*.js|*.ts|*.tsx|*.jsx|*.java|*.kt|*.rs|*.c|*.h|*.cpp|*.hpp|*.cs|*.swift)
          add_c_style_header "$file"
          ;;

        *.sh|*.bash|*.zsh|*.py|*.rb|*.pl|Dockerfile|*.dockerfile|Makefile)
          add_line_comment_header "$file" "#"
          ;;

        *.sql|*.lua)
          add_line_comment_header "$file" "--"
          ;;
      esac
    done