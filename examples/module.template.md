# SAMPLE: module の説明

記入用テンプレート。実際の module ではない。README の先頭に [metadata](knowledge.template.md) を付け、kind を module にする。

## 用途と API

解決する問題、既存 module で足りない理由、公開 API、引数・戻り値・エラーを記録する。

## 境界と並行性

ゼロ値・nil・上限・キャンセル・競合の契約を定め、共有状態の所有者を説明する。

## 検証と由来

unit・edge・race/concurrency test、実行できる eval とそのコマンドを示す。PROVENANCE.md と module.json を添える。実例は [contextwait](../modules/go/contextwait/README.md) を参照する。
