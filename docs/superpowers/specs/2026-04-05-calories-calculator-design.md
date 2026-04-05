# カロリー計算機 設計ドキュメント

Date: 2026-04-05

## 概要

ユーザーが日々の摂取カロリー・運動消費カロリー・お菓子カロリーを記録し、基準値との差分を管理するWebアプリケーション。排泄・体重計測のタイムスタンプ記録、月間集計、ユーザーごとのデータ管理機能を持つ。

---

## 全体アーキテクチャ

- **フロントエンド**：Vue.js 3（CDN経由、ビルド不要）によるSPA
- **バックエンド**：PHP 8.x の REST API（JSON返却）
- **データ**：SQLite（さくらウェブサーバー上のファイル）
- **認証**：PHPネイティブセッション（`PHPSESSID` クッキー）
- **デプロイ先**：さくらのウェブサービス（Apache + PHP）

### ディレクトリ構成

```
/
├── index.html          # Vue.js SPA エントリーポイント
├── api/
│   ├── auth.php        # ログイン・ログアウト・登録
│   ├── daily.php       # 日別カロリー記録のCRUD
│   ├── settings.php    # 基準カロリー設定（期間管理）
│   ├── events.php      # 排泄・体重計測記録
│   └── monthly.php     # 月間集計データ
├── js/
│   └── app.js          # Vue.js アプリケーション本体
└── db/
    ├── calories.db     # SQLite データベース
    └── .htaccess       # DBへの直接HTTPアクセスを禁止
```

---

## データモデル

### users
```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL
);
```

### daily_records
```sql
CREATE TABLE daily_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  date TEXT NOT NULL,           -- "YYYY-MM-DD"
  intake_kcal INTEGER,          -- 摂取カロリー
  exercise_kcal INTEGER,        -- 運動消費カロリー
  snack_kcal INTEGER,           -- お菓子カロリー
  memo TEXT,
  UNIQUE(user_id, date),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
```

### daily_events
```sql
CREATE TABLE daily_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  recorded_at TEXT NOT NULL,    -- "YYYY-MM-DD HH:MM:SS"
  event_type TEXT NOT NULL,     -- "excretion" or "weigh_in"
  date TEXT NOT NULL,           -- 所属する日付（朝5時ルール適用済み）
  FOREIGN KEY (user_id) REFERENCES users(id)
);
```

### calorie_settings
```sql
CREATE TABLE calorie_settings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  start_date TEXT NOT NULL,     -- "YYYY-MM-DD"
  end_date TEXT,                -- NULL = 現在も有効
  base_intake_kcal INTEGER NOT NULL,
  base_exercise_kcal INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id)
);
```

---

## API設計

全エンドポイントはセッション未認証時に `{"error": "Unauthorized"}` と HTTP 401 を返す（auth.php 除く）。

| エンドポイント | メソッド | 機能 |
|---|---|---|
| `api/auth.php?action=register` | POST | ユーザー登録 |
| `api/auth.php?action=login` | POST | ログイン |
| `api/auth.php?action=logout` | POST | ログアウト |
| `api/auth.php?action=me` | GET | 現在のユーザー情報取得 |
| `api/daily.php` | GET | 指定日の記録取得（`?date=YYYY-MM-DD`） |
| `api/daily.php` | POST | 記録の作成・更新（upsert） |
| `api/events.php` | GET | 指定日のイベント一覧取得（`?date=YYYY-MM-DD`） |
| `api/events.php` | POST | 排泄・体重計測を記録（現在時刻で自動記録） |
| `api/events.php` | DELETE | イベント削除（`?id=N`） |
| `api/settings.php` | GET | 基準カロリー設定一覧取得 |
| `api/settings.php` | POST | 期間設定の追加（`id`なし）・編集（`id`あり） |
| `api/settings.php` | DELETE | 期間設定の削除（`?id=N`） |
| `api/monthly.php` | GET | 月間集計（`?year=YYYY&month=MM`） |

### 月間集計レスポンス例
```json
{
  "year": 2026,
  "month": 4,
  "total_diff_kcal": -2100,
  "avg_snack_kcal": 85,
  "days_recorded": 20
}
```

### 期間重複ルール

- `calorie_settings` の期間（start_date〜end_date）は同一ユーザー内で重複してはならない
- API側で重複チェックを行い、重複する場合は `{"error": "期間が重複しています"}` を返す
- ある日付の基準値を参照する際は、`start_date <= date AND (end_date IS NULL OR end_date >= date)` で一意に決まる

---

## フロントエンド画面構成

### 画面一覧

```
ログイン画面       → 未認証時のみ表示
登録画面           → ログイン画面からリンク
─────────────────────────────────────
メイン画面（認証後）
├── 今日の記録      → デフォルト表示
│                    摂取・運動・お菓子kcal入力
│                    メモ入力
│                    排泄ボタン・体重計測ボタン（ワンタップで現在時刻を記録）
│                    その日の差分（+/-kcal）をリアルタイム表示
├── 日別一覧        → リスト形式で過去の記録を確認・編集
├── 月間サマリー    → 月間累計差分、平均お菓子カロリーを表示
└── 設定            → 基準カロリーの期間設定を管理
```

### 差分計算ロジック

```
差分 = (摂取kcal - 基準摂取kcal) - (運動kcal - 基準消費kcal)
     = 摂取kcal - 基準摂取kcal - 運動kcal + 基準消費kcal
```

例：基準摂取1500、基準消費300、実際摂取1600、実際運動300 → 差分 = +100

### 朝5時ルール

現在時刻が00:00〜04:59の場合、対象日付を前日として扱う。PHPの `api/` 側で計算して返す。

---

## セキュリティ・エラー処理

### セキュリティ
- パスワードは `password_hash()` / `password_verify()` を使用
- SQLはPDOプリペアドステートメントのみ使用（SQLインジェクション対策）
- セッションIDはログイン時に `session_regenerate_id(true)` で再生成
- `db/.htaccess` でSQLiteファイルへの直接HTTPアクセスを禁止
- `api/` の各PHPファイルは `Content-Type: application/json` のみ返す

### エラー処理
- API側：エラー時は `{"error": "メッセージ"}` 形式でJSONを返す
- フロント側：API呼び出し失敗時は画面上部にエラーメッセージを表示（3秒で自動消去）
- 入力バリデーション：カロリーは0以上の整数のみ許可（フロント・バックエンド両方で検証）

---

## 技術スタック

| 項目 | 採用技術 |
|---|---|
| フロントエンド | Vue.js 3 (CDN) |
| バックエンド | PHP 8.x |
| DB | SQLite 3 |
| 認証 | PHPセッション |
| デプロイ | さくらのウェブサービス（Apache） |
