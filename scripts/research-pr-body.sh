#!/bin/bash
# Render the shared template after the runner's local validation has succeeded.
# Read committed artifacts only; never invoke models, change Git state, or claim CI success.
set -Eeuo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
[[ "$#" == 3 ]] || { printf 'usage: %s TOPIC BASE_SHA PARTIAL\n' "$0" >&2; exit 2; }
topic="$1" base="$2" partial="$3"
[[ "$base" =~ ^[0-9a-f]{40}$ && "$partial" =~ ^[01]$ ]] || exit 2
head="$(git rev-parse HEAD)"
changed="$(git diff --name-only "$base" "$head" -- knowledge/ sources/catalog/ evals/knowledge/)"
[[ -n "$changed" ]] || { printf 'no committed research artifacts\n' >&2; exit 2; }
sources=''
while IFS= read -r path; do
  case "$path" in
    knowledge/*.md)
      # The metadata contract uses JSON between the first two --- lines.
      refs="$(git show "$head:$path" | awk '/^---$/ { boundary++; next } boundary == 1 { print }' |
        jq -er '.sources[] | "- [\(.id)](\(.url))"')"
      sources="${sources}${refs}"$'\n'
      ;;
  esac
done <<< "$changed"
[[ -n "$sources" ]] || { printf 'no source references in changed knowledge\n' >&2; exit 2; }

# Headings come from the same template used by GitHub and GPT. Fail closed if a
# future template section is unsupported instead of silently dropping it.
sections=0
seen="|"
while IFS= read -r line; do
  case "$line" in
    '## '*)
      [[ "$seen" != *"|$line|"* ]] || { printf 'duplicate PR template section: %s\n' "$line" >&2; exit 2; }
      seen="$seen$line|"
      printf '%s\n\n' "$line"
      ;;
    *) continue ;;
  esac
  case "$line" in
    '## 概要')
      printf 'OpenCode Research Pipeline による調査結果の更新です。\n\n対象テーマ: %s\n' "$topic"
      ;;
    '## 変更内容')
      while IFS= read -r path; do printf -- '- `%s`\n' "$path"; done <<< "$changed"
      printf '\n適用版と調査範囲は各 knowledge 文書の metadata・本文に記録しています。\n'
      ;;
    '## 出典')
      printf '%s' "$sources" | sort -u
      printf '\n版・取得日・ライセンスは参照先の `sources/catalog/<id>.json` に記録しています。\n'
      ;;
    '## 検証結果')
      printf -- '- 対象: baseline `%s` / PR 作成時 head `%s`\n' "$base" "$head"
      printf '%s\n' \
        '- 成功: commit 前のローカル検証 `just validate`・`just index`・`just check`・`git diff --check`・staged diff check' \
        '- 失敗: 上記の最終ローカル検証では該当なし' \
        '- 未実行: マージ後の main に対する検証（PR 作成時点では未マージ）' \
        '- CI: 未確認（PR 作成時点）。対象 head の Linux / macOS の結果と run / job URL を確認してください。head 更新後は以前の検証結果を引き継がないでください。'
      ;;
    '## 未確認事項')
      if [[ "$partial" == 1 ]]; then
        printf -- '- 部分完了: 成功した worker の成果だけを含みます。未完了のテーマと失敗理由は run の記録を確認してください。\n'
      fi
      printf -- '- 各 knowledge 文書に記載した未確認事項・適用条件を参照してください。形式検証は記述の真実性や人間のレビュー完了を保証しません。\n'
      ;;
    *) printf 'unsupported PR template section: %s\n' "$line" >&2; exit 2 ;;
  esac
  sections=$((sections + 1))
  printf '\n'
done < "$ROOT/.github/pull_request_template.md"
[[ "$sections" == 5 ]] || { printf 'incomplete PR template\n' >&2; exit 2; }
