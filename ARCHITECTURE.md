# Architecture

## 概要

Goバックエンド（REST API、CGIとして動作）+ Vue 3フロントエンド（SPA）の構成。DBはSQLite。さくらのレンタルサーバー（FreeBSD 11 / Apache）で運用。

## ファイル構成

```
/
├── index.html          # SPAエントリポイント、CSS、Vueマウント
├── js/app.js           # Vueアプリ全体（コンポーネント定義からマウントまで）
├── src/                # Go API のソース（package main）
│   ├── main.go         # CGI/ローカルサーバー起動、ファイル名でのルーティング
│   ├── util.go         # JSONレスポンス、リクエスト値の変換、todayDate()
│   ├── db.go           # SQLite接続、テーブル自動作成
│   ├── auth.go         # 認証エンドポイント、セッション管理
│   ├── daily.go        # 日次記録 CRUD
│   ├── events.go       # イベント（排泄・体重）CRUD
│   ├── monthly.go      # 月間集計（GET のみ）
│   ├── settings.go     # 基準カロリー設定 CRUD
│   └── api_settings.go # 外部API設定 GET/POST
├── api/
│   ├── api.cgi         # ビルド成果物（.gitignoreで除外）
│   └── .htaccess       # api/*.php へのリクエストを api.cgi に内部rewrite
├── db/
│   └── calories.db     # SQLiteファイル（.gitignoreで除外）
├── Makefile            # build: FreeBSD向けビルド / dev: ローカルサーバー
└── .htaccess           # mod_rewrite（SPA）
```

## ビルド・デプロイ

FreeBSD 11 で動かすため Go 1.18（`~/go/bin/go1.18.10`）でビルドする。`golang.org/x/sys` は `go.mod` の `replace` で FreeBSD 11 互換の版に固定している。

```
make build   # api/api.cgi を生成
make dev     # http://localhost:8080/ でローカル実行（DBはカレントの db/calories.db）
```

`api/api.cgi`（755）と `api/.htaccess` をサーバーの `api/` に置けば動く。DBパスは `api.cgi` の親ディレクトリ基準で `db/calories.db`。

URLは PHP 時代の `api/*.php` のまま（フロントエンド変更なし）。`.htaccess` で内部rewriteし、Go 側は `REQUEST_URI` のファイル名でルーティングする。

## フロントエンドコンポーネント（js/app.js）

| コンポーネント | 役割 |
|---|---|
| `LoginView` | ログイン・新規登録フォーム |
| `TodayView` | 日次記録の表示・編集、前後日ナビ、外部API取得、5分ポーリング |
| `ListView` | 直近30日の記録一覧・インライン編集 |
| `MonthlyView` | 月間集計サマリー |
| `SettingsView` | 基準カロリー期間管理、外部API設定 |

ルーターは使わず、メインアプリの `page` ref でコンポーネントを切り替える。

## APIエンドポイント一覧

### auth.php

| メソッド | クエリ | 概要 |
|---|---|---|
| POST | `?action=register` | 新規登録、セッション開始 |
| POST | `?action=login` | ログイン、セッション開始 |
| POST | `?action=logout` | セッション破棄 |
| GET | `?action=me` | セッション確認、ユーザー情報返却 |

### daily.php（要認証）

| メソッド | パラメータ | 概要 |
|---|---|---|
| GET | `?date=YYYY-MM-DD` | 指定日の記録＋基準カロリー設定を返す。dateなしは今日 |
| POST | body: `{date, intake_kcal, exercise_kcal, snack_kcal, memo}` | UPSERT |

GETレスポンス: `{ date, today, record, setting }`

### events.php（要認証）

| メソッド | 概要 |
|---|---|
| GET `?date=` | 指定日のイベント一覧（recorded_at昇順） |
| POST | `{event_type: "excretion"|"weigh_in", date}` でイベント追加 |
| PUT | `{id, time: "HH:MM"}` で時刻変更 |
| DELETE `?id=` | イベント削除 |

### settings.php（要認証）

基準カロリー設定（`calorie_settings`テーブル）のCRUD。期間重複はサーバー側でチェックしてHTTP 409を返す。

### monthly.php（要認証）

GET `?year=&month=` → `{ total_diff_kcal, avg_snack_kcal, avg_intake_kcal, avg_exercise_kcal, avg_diff_kcal, days_recorded }`

### api_settings.php（要認証）

外部カロリー取得APIのURL・トークンをユーザーごとに保存。GET/POST のみ。

## DBスキーマ

テーブル作成は `src/db.go` の `openDB()` 内で `CREATE TABLE IF NOT EXISTS` により自動実行される。マイグレーション機構はない（テーブル追加時は `CREATE TABLE IF NOT EXISTS` で追記する）。

```sql
users (id, username UNIQUE, password_hash, created_at)

daily_records (id, user_id, date, intake_kcal, exercise_kcal, snack_kcal, memo)
  UNIQUE(user_id, date)  -- UPSERT の根拠

daily_events (id, user_id, recorded_at, event_type, date)
  event_type: "excretion" | "weigh_in"

calorie_settings (id, user_id, start_date, end_date NULL=無期限, base_intake_kcal, base_exercise_kcal)
  -- 期間重複チェックはsettings.goのhasOverlap()で実施

user_api_settings (user_id PK, api_url, api_token)
  -- UPSERT で常に1ユーザー1レコード

sessions (id PK = トークンのSHA-256, user_id, username, expires_at UNIX秒)
```

## 認証

SQLiteの `sessions` テーブルによる独自セッション。cookie名は `calories_sid`（HttpOnly、SameSite=Lax、HTTPS時Secure）。有効期限はログイン時から30日。パスワードはbcrypt（PHPの `password_hash` で作った `$2y$` 形式もそのまま検証可能）。`requireAuth()` は未認証時にHTTP 401を返す。全APIエンドポイント（`auth.php`の`register`/`login`/`logout`を除く）が `requireAuth()` を呼ぶ。

## 差分計算ロジック

```
差分 = 摂取kcal - 基準摂取kcal - 運動消費kcal + 基準消費kcal
```

正値（赤）= オーバー、負値（緑）= 目標以内。フロントエンドの `diff` computed と `monthly.go` の集計で同じ式を使う。

## 「朝5時ルール」

`src/util.go` の `todayDate()` で実装。JST固定で、00:00〜04:59は前日扱いにする（深夜の記録を前日に紐づける）。

## 外部API連携

`TodayView` の「自動取得」ボタンから、ユーザーが設定したURLに対して以下のリクエストを投げる：

```
GET {api_url}?action=calories&token={api_token}&date={date}
```

レスポンス形式: `{ intake_kcal, burn_kcal, snack_kcal }`（nullは無視）

フロントから直接fetchする（サーバーを経由しない）。CORSはユーザーが用意する外部APIの責務。

## ポーリング

`TodayView` の `onMounted` で `setInterval(5分)` を張り、選択中の日付を自動リロードする。`v-if` によるアンマウントで `clearInterval` が呼ばれる。

## 注意点・制約

- SQLite なので同時書き込みに弱い。個人利用前提。
- テーブルに `ALTER TABLE` が必要な変更は、現状手動で対応（マイグレーション機構なし）。
- `db/calories.db` は `.gitignore` で除外。デプロイ時は `db/` ディレクトリのパーミッションを書き込み可能にする必要がある。
- Vue はCDN（unpkg）から読み込む（ビルドステップなし）。
