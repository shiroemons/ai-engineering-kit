# 参照ライセンスを記録する

ライセンスは source ごとに確認し、識別子と根拠を残す。MIT、Apache-2.0、BSD（3条項）、GPL、AGPL は別のライセンスとして扱う。GPL と AGPL はバージョンや only / or-later の区別も原文どおりに記録する。

名称だけで利用条件を判断しない。対象 commit の LICENSE、ファイルの notice、例外を確認し、直接使用・改変の範囲を PROVENANCE.md に残す。unknown のコードは module へ持ち込まない。

seed の Go API は [Go LICENSE](https://go.dev/LICENSE) と公式 package ページの BSD ライセンス（3条項）の表示を2026-09-23に確認した。API の説明だけを参照し、外部コードの転記はしていない。正式な識別子は catalog に記録する。

本リポジトリの配布ライセンスは未選定。ここでの参照資料の識別は、配布ライセンスの採用を意味しない。
