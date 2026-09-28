---
{
  "id": "security-asvs-password-storage-argon2id-pepper",
  "title": "OWASP ASVS-based password storage with Argon2id, bcrypt limits, salt, pepper, and verifier leak protection",
  "kind": "knowledge",
  "technology": "security",
  "version": "OWASP Password Storage Cheat Sheet (current) + RFC 9106 (Sep 2021) + NIST SP 800-63B-4 (1 Aug 2025) + OWASP ASVS v5.0.0",
  "tags": ["research-domain:security", "security", "password-storage", "password-hashing", "argon2id", "bcrypt", "scrypt", "pbkdf2", "salt", "pepper", "verifier", "leak-protection", "work-factor", "asvs"],
  "sources": [{"id": "owasp-password-storage-cheat-sheet", "url": "https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html", "type": "official_docs"}, {"id": "rfc9106-argon2", "url": "https://www.rfc-editor.org/rfc/rfc9106.html", "type": "official_docs"}, {"id": "nist-sp800-63b-4", "url": "https://pages.nist.gov/800-63-4/sp800-63b.html", "type": "official_docs"}, {"id": "owasp-asvs-v5-project", "url": "https://owasp.org/www-project-application-security-verification-standard/", "type": "official_docs"}],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# OWASP ASVS-based password storage with Argon2id, bcrypt limits, salt, pepper, and verifier leak protection

OWASP ASVS v5.0.0 を検証の枠組み、OWASP Password Storage Cheat Sheet を現行のパラメータ選択、RFC 9106 を Argon2 の規定、NIST SP 800-63B-4 を verifier 保管の要件とする password storage の要点。対象は password hashing scheme の選択、salt、pepper、verifier の保管と漏洩時の被害軽減である。

## 要点 (文書化された事実)

### Argon2id を優先する (OWASP Password Storage Cheat Sheet, current)

- OWASP Password Storage Cheat Sheet は Argon2id を第一選択とし、記載の動作点は `m=19456 (19 MiB)・t=2・p=1` である。
- 同等な memory/time トレードオフとして `scrypt N=2^17・r=8・p=1` と `PBKDF2-HMAC-SHA256 600k 回` を挙げる。
- bcrypt は legacy のみとし、work factor は 10 以上、72-byte の入力制限があることを明記する。
- bcrypt を使い続ける場合は base64 と HMAC による pre-hash で null-byte truncation と password shucking に対処する。
- work factor は次回ログイン時にアップグレードする (upgrade on next login)。

### Argon2 の規定と推奨動作点 (RFC 9106, Sep 2021, Informational/CFRG)

- RFC 9106 は Argon2 v1.3 を定義し、Argon2id のサポートを MUST、Argon2d と Argon2i のサポートを MAY とする。
- 入力は password (p)、tag length (T)、memory (m)、iterations (t)、salt、secret である。secret は pepper 相当の鍵入力として使える。
- salt は 16-byte が RECOMMENDED であり、パスワード毎に一意である SHOULD (SHOULD be unique per password) とする。
- Argon2id はハイブリッド構成であり、first pass の前半を Argon2i、以後の処理を Argon2d で行う。
- RFC の推奨動作点は FIRST RECOMMENDED `t=1・p=4・m=2 GiB` と SECOND RECOMMENDED `t=3・p=4・m=64 MiB` である。これらは OWASP Cheat Sheet の `m=19456・t=2・p=1` とは異なる動作点であり、同一値ではない。memory と time のどちらに寄せるかの選択であり、運用環境のメモリと並列度に合わせて選ぶ。

### salt・hash・scheme の保管と cost (NIST SP 800-63B-4, 1 Aug 2025)

- NIST SP 800-63B-4 は password を SP 800-132 に沿った suitable password hashing scheme で salted+hashed して保管することを SHALL とする。本書は SP 800-63B (June 2017) の後継版である。
- salt は 32-bit 以上 (SHALL be at least 32 bits) で衝突を最小化し、salt・hash・scheme の参照を保管する (SHALL)。
- cost factor は実用的な上限まで高くする SHOULD (as high as practical) とする。具体的な数値は本書の確認範囲になく、OWASP Cheat Sheet または RFC 9106 の動作点を参照する。
- blocklist との照合および rate-limiting を verifier の運用要件として含める。

### pepper (secret-key) の分離保管 (OWASP + NIST)

- OWASP Password Storage Cheat Sheet はパスワード毎の一意な salt に加え、共有の pepper を vault または HSM に分離保管する。pre-hash と post-hash の HMAC という選択肢を示す。
- NIST SP 800-63B-4 は verifier が secret-key (pepper) による反復を追加することを SHOULD とし、その鍵を分離保管し、理想的には HSM または TEE に置くとする。
- 両者は一致して「salt は verifier と一緒に置くが、pepper は別系統に置く」という構造を求める。pepper を password hash と同じテーブルに置くことは分離にならない。

### ASVS の位置づけ (OWASP ASVS Project page, v5.0.0)

- OWASP ASVS Project page は v5.0.0 が現行 stable であることを示し、ASVS を技術的セキュリティ制御のテストと secure-development 要件の基盤とする。
- 要件は `v5.0.0-x.y.z` 形式で versioned 参照でき、CSV/JSON の要件一覧が提供される。
- 本書は ASVS を検証の枠組みとして使い、password hashing の数値自体は OWASP Cheat Sheet・RFC 9106・NIST SP 800-63B-4 から取る。本書の確認範囲では ASVS 側の password storage 個別要件番号は主張しない。

## 推奨方法 (上記からの設計上のまとめ)

- 新規は Argon2id を選ぶ。OWASP の `m=19456・t=2・p=1` を出発点とし、サーバのメモリとログイン時レイテンシを測って memory/time を調整する。RFC 9106 の FIRST/SECOND RECOMMENDED は別の運用点であり、そのまま混ぜない。
- salt は CSPRNG でパスワード毎に一意に生成し (RFC 9106 の 16-byte RECOMMENDED と NIST の 32-bit 以上 SHALL を両方満たす長さを取る)、salt・hash・scheme 識別子を verifier と一緒に保管する。
- pepper は vault または HSM/TEE に分離保管し、OWASP の pre-hash または post-hash HMAC のいずれか一つの方式に統一する。鍵 rotation の手順を用意する。
- bcrypt 既存分は work factor 10 以上を保ち、72-byte 制限・null-byte truncation・password shucking への対策として base64+HMAC pre-hash を適用し、次回ログイン時に Argon2id へ移行または work factor を引き上げる。
- verifier 保管に加え、blocklist 照合と rate-limiting を認証経路に入れる。漏洩時は pepper の分離により offline 解読の難度を保ちつつ、影響範囲の verifier を再ハッシュ計画に入れる。

## 避ける使い方

- 平文、 reversible encryption、可逆変換だけでの password 保管。NIST の salted+hashed (SHALL) に反する。
- 全 password 共通の salt、短い salt、salt の未保管。RFC 9106 の一意性 (SHOULD unique) と NIST の 32-bit 以上・保管 (SHALL) に反する。
- pepper を password hash と同じ DB 行・同じ設定ファイルに置くこと。OWASP と NIST の分離保管の要件を満たさない。
- bcrypt の 72-byte 制限を無視した長い password の素通し、null-byte truncation の未対策。password shucking の原因になる。
- cost factor を初期値のまま固定し、upgrade on next login の仕組みを持たないこと。計算能力の向上に対して検証強度が下がる。
- Argon2d または Argon2i を新規に選ぶこと。RFC 9106 は Argon2id を MUST、他を MAY としており、OWASP も Argon2id を第一選択とする。
- OWASP の動作点と RFC の動作点の数値を根拠なく混ぜること (例: RFC の memory に OWASP の t を付ける)。memory/time トレードオフの前提が崩れる。

## 適用版と本番での注意

- `OWASP Password Storage Cheat Sheet (current, 2026-09-28 取得)`、`RFC 9106 (Informational, CFRG, September 2021, Argon2 v1.3)`、`NIST SP 800-63B-4 (1 Aug 2025, SP 800-63B June 2017 の後継)`、`OWASP ASVS v5.0.0 (latest stable)` で確認した。将来も最新とは扱わない。
- 本文は `expires_at` 2026-12-27 (official_docs TTL 90 日)。
- Argon2 の memory・parallelism は DoS 耐性とログイン時レイテンシに直結する。本番投入前に実機でメモリ使用量と並列ログイン時の CPU/メモリを測定し、rate-limiting と合わせて調整する必要がある。
- **未確認**: 各言語の Argon2id 実装が RFC 9106 の secret (pepper 入力) に対応しているか、HSM/TEE での pepper 運用の具体手順、PBKDF2 回数の将来改定、SP 800-132 の詳細要件。本文の範囲を超えるため、実装前に対応ライブラリと NIST 原文の公式文書で確認する必要がある。
