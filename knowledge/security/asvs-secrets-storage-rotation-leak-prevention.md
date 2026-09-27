---
{
  "id": "security-asvs-secrets-storage-rotation-leak-prevention",
  "title": "OWASP ASVS-based secrets handling with vault storage, rotation, and leak prevention",
  "kind": "knowledge",
  "technology": "security",
  "version": "OWASP ASVS v5.0.0 (stable May 2025, tag v5.0.0) + OWASP Cheat Sheet Series (current)",
  "tags": ["research-domain:security", "security", "secrets", "vault", "storage", "rotation", "expiry", "revocation", "leak", "least-privilege", "csprng", "entropy", "hsm", "kek", "inventory", "audit", "asvs", "cheat-sheet"],
  "sources": [{"id": "owasp-asvs-v5-configuration", "url": "https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x22-V13-Configuration.md", "type": "official_docs"}, {"id": "owasp-asvs-v5-cryptography", "url": "https://raw.githubusercontent.com/OWASP/ASVS/v5.0.0/5.0/en/0x20-V11-Cryptography.md", "type": "official_docs"}, {"id": "owasp-secrets-management-cheat-sheet", "url": "https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html", "type": "official_docs"}, {"id": "owasp-key-management-cheat-sheet", "url": "https://cheatsheetseries.owasp.org/cheatsheets/Key_Management_Cheat_Sheet.html", "type": "official_docs"}],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# OWASP ASVS-based secrets handling with vault storage, rotation, and leak prevention

OWASP ASVS v5.0.0 の V13 Configuration と V11 Cryptography、および OWASP Cheat Sheet Series の Secrets Management / Key Management に基づく secrets の保管・rotation・漏洩防止の要点。対象は application secrets (API keys、credentials、tokens) と cryptographic keys の lifecycle (creation、storage、rotation、revocation、expiration、destruction) である。

## 要点 (文書化された事実)

### vault による保管とコード混入禁止 (ASVS V13, 13.3.1 / 13.3.2)

- 13.3.1 は secrets の creation、storage、access、destruction を vault で行うことを求め、code や build artifacts に secrets を含めてはならないとする。L3 では HSM の利用が要件に含まれる。
- 13.3.2 は vault への access に least privilege を適用することを求める。
- Secrets Management Cheat Sheet も secrets の centralize と standardize、および least-privilege の access control を指針とする。

### expiry・rotation・文書化 (ASVS V13, 13.3.4 / 13.1.4)

- 13.3.4 は文書化 (documentation) に沿った secrets の expiry と rotation を求める。
- 13.1.4 は critical secrets と rotation schedule の文書化を求める。
- Secrets Management Cheat Sheet は lifecycle stages として creation、rotation、revocation、expiration を挙げ、sidecar や Lambda による automated rotation の pattern を示す。break-glass 用の backup と restore も指針に含まれる。

### backend 認証は静的 secrets を避ける (ASVS V13, 13.2.1)

- 13.2.1 は backend 間の認証に service accounts、short-term tokens、certificates を使うことを求め、static passwords や API keys は使わないとする。

### 生成と鍵管理 (ASVS V11 / Key Management Cheat Sheet)

- 11.5.1 は推測不能 (non-guessable) な secrets の生成に CSPRNG を使い、少なくとも 128-bit の entropy を持たせることを求める。
- 11.1.1 は NIST SP 800-57 に沿った key-management policy と key lifecycle の文書化を求める。ただし oversharing (文書への書き込みすぎ) を避ける条件が付く。
- 11.1.2 は keys、algorithms、certs と usage limits を cryptographic inventory として把握することを求める。
- 11.2.1 は industry-validated な実装の利用、11.2.2 は key replacement と re-encryption を含む crypto agility を求める。
- Key Management Cheat Sheet は鍵を plaintext で保管してはならないとし、cryptographic vault または HSM に integrity protection 付きで保管することを指針とする。export する鍵は同等以上の強度の KEK (key-encrypting key) で暗号化する。生成は FIPS module で行い、配布は安全な方法で行う。escrow と backup、accountability と audit、compromise 時の recovery plan も指針に含まれる。

### 監査・通信保護・漏洩対応 (Cheat Sheets)

- Secrets Management Cheat Sheet は TLS everywhere、auditing、CI/CD hardening と detection、および leak 時の incident-response の節を持つ。
- Key Management Cheat Sheet は accountability と audit を指針とする。

## 推奨方法 (上記からの設計上のまとめ)

- secrets は vault に集約し、application からは実行時に least-privilege で取得する。code、build artifacts、container image、log に secrets を埋め込まない。backend 間認証は static passwords や long-lived API keys ではなく service accounts、short-term tokens、certificates を使う。
- critical secrets を列挙し、rotation schedule と expiry を文書化する。rotation は手動手順ではなく automated rotation (sidecar や Lambda などの pattern) で回し、revocation と expiration まで lifecycle として管理する。break-glass 用の backup と restore 手順を用意する。
- secrets と鍵の生成は CSPRNG で行い、推測不能な値には 128-bit 以上の entropy を確保する。暗号実装は自作せず industry-validated なものを使い、key replacement と re-encryption が可能な crypto agility を保つ。
- 鍵は plaintext で置かず、vault または HSM に integrity protection 付きで保管する。export が必要な鍵は同等以上の強度の KEK で暗号化する。keys、algorithms、certs と usage limits の inventory を維持し、key-management policy と lifecycle を文書化する (詳細の書き込みすぎには注意する)。
- secrets への access と lifecycle 操作を audit し、取得・利用の通信は TLS で保護する。CI/CD では hardening と detection を行い、leak が起きた場合は incident-response と compromise-recovery plan に従って revocation と rotation を行う。

## 避ける使い方

- secrets を code、設定ファイルの平文、build artifacts、container image、log に残すこと。13.3.1 に反する。
- vault への広い権限付与や共有 credentials の使い回し。13.3.2 の least privilege に反する。
- rotation schedule や expiry のない long-lived な static passwords・API keys の常用。13.3.4 と 13.2.1 に反する。
- 推測可能な値や弱い乱数による secrets 生成。11.5.1 の CSPRNG と 128-bit entropy の要件に反する。
- 鍵の plaintext 保管、KEK なしの export、強度不足の KEK での暗号化。Key Management Cheat Sheet の指針に反する。
- 自作の暗号実装の採用や、鍵置換・再暗号化の手段がない設計。11.2.1 と 11.2.2 に反する。
- leak 検知の仕組みや incident-response・recovery plan がない運用。Cheat Sheet の detection と incident-response の節の想定外になる。

## 適用版と本番での注意

- `OWASP ASVS v5.0.0 (stable May 2025, tag v5.0.0)` の V13 Configuration と V11 Cryptography、および OWASP Cheat Sheet Series の Secrets Management / Key Management の現行ページ (2026-09-27 取得) で確認した。将来も最新とは扱わない。
- 本文は `expires_at` 2026-12-26 (official_docs TTL 90 日)。
- ASVS の要件番号 (13.x.x、11.x.x) は検証対象の requirement 識別子であり、L1/L2/L3 の適用 level は本書では HSM (13.3.1 の L3 要件) を除き主張しない。調達・監査で level が問題になる場合は ASVS 原文の level 列を確認する必要がある。
- **未確認**: 各 vault 製品・HSM・CI/CD secret 機能が上記要件を満たすかの対応表、sidecar・Lambda pattern の具体的な実装手順、rotation 間隔の推奨値 (ASVS は文書化を求めるが間隔の値自体は本書の確認範囲にない)。実装前に対応製品の公式文書で確認する必要がある。
