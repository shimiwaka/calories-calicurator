# カロリー計算機 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ユーザーが日々の摂取・運動・お菓子カロリーを記録し、基準値との差分を管理できるWebアプリケーションを構築する。

**Architecture:** PHP 8.x REST API（SQLite）+ Vue.js 3 SPA（CDN経由）。PHPセッションで認証管理。さくらのウェブサービス（Apache）にデプロイ。

**Tech Stack:** PHP 8.x, SQLite 3, Vue.js 3 (CDN), Apache (.htaccess)

---

## ファイル構成

```
/
├── index.html              # Vue.js SPA エントリーポイント
├── js/
│   └── app.js              # Vue.jsアプリ本体（全画面・ロジック）
├── api/
│   ├── _db.php             # DB接続・初期化ヘルパー（共通）
│   ├── _auth_check.php     # セッション認証チェック（共通）
│   ├── auth.php            # 登録・ログイン・ログアウト・me
│   ├── daily.php           # 日別記録のGET/POST
│   ├── events.php          # 排泄・体重計測のGET/POST/DELETE
│   ├── settings.php        # 基準カロリー期間設定のGET/POST/DELETE
│   └── monthly.php         # 月間集計のGET
└── db/
    └── .htaccess            # SQLiteファイルへの直接アクセス禁止
```

---

## Task 1: プロジェクト基盤（DB初期化・共通ヘルパー）

**Files:**
- Create: `db/.htaccess`
- Create: `api/_db.php`
- Create: `api/_auth_check.php`

- [ ] **Step 1: `db/.htaccess` を作成してSQLiteへのHTTPアクセスを禁止する**

```
# db/.htaccess
Deny from all
```

- [ ] **Step 2: `api/_db.php` を作成する**

```php
<?php
// api/_db.php
// DBファイルのパスはweb公開ディレクトリの外が理想だが、
// さくら共有ホスティングではdocument root配下に置き.htaccessで保護する
function get_db(): PDO {
    $db_path = __DIR__ . '/../db/calories.db';
    $db_dir = dirname($db_path);
    if (!is_dir($db_dir)) {
        mkdir($db_dir, 0755, true);
    }
    $pdo = new PDO('sqlite:' . $db_path);
    $pdo->setAttribute(PDO::ATTR_ERRMODE, PDO::ERRMODE_EXCEPTION);
    $pdo->exec('PRAGMA foreign_keys = ON;');
    $pdo->exec("
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT UNIQUE NOT NULL,
            password_hash TEXT NOT NULL,
            created_at TEXT NOT NULL
        );
        CREATE TABLE IF NOT EXISTS daily_records (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            date TEXT NOT NULL,
            intake_kcal INTEGER,
            exercise_kcal INTEGER,
            snack_kcal INTEGER,
            memo TEXT,
            UNIQUE(user_id, date),
            FOREIGN KEY (user_id) REFERENCES users(id)
        );
        CREATE TABLE IF NOT EXISTS daily_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            recorded_at TEXT NOT NULL,
            event_type TEXT NOT NULL,
            date TEXT NOT NULL,
            FOREIGN KEY (user_id) REFERENCES users(id)
        );
        CREATE TABLE IF NOT EXISTS calorie_settings (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            start_date TEXT NOT NULL,
            end_date TEXT,
            base_intake_kcal INTEGER NOT NULL,
            base_exercise_kcal INTEGER NOT NULL,
            FOREIGN KEY (user_id) REFERENCES users(id)
        );
    ");
    return $pdo;
}

// 朝5時ルール：00:00〜04:59は前日扱い
function get_today_date(): string {
    $hour = (int)date('H');
    if ($hour < 5) {
        return date('Y-m-d', strtotime('-1 day'));
    }
    return date('Y-m-d');
}
```

- [ ] **Step 3: `api/_auth_check.php` を作成する**

```php
<?php
// api/_auth_check.php
function require_auth(): int {
    if (session_status() === PHP_SESSION_NONE) {
        session_start();
    }
    if (empty($_SESSION['user_id'])) {
        http_response_code(401);
        echo json_encode(['error' => 'Unauthorized']);
        exit;
    }
    return (int)$_SESSION['user_id'];
}

function json_response(mixed $data, int $status = 200): void {
    http_response_code($status);
    header('Content-Type: application/json; charset=utf-8');
    echo json_encode($data, JSON_UNESCAPED_UNICODE);
    exit;
}
```

- [ ] **Step 4: コミットする**

```bash
git add db/.htaccess api/_db.php api/_auth_check.php
git commit -m "feat: add DB helper and auth check middleware"
```

---

## Task 2: 認証API（auth.php）

**Files:**
- Create: `api/auth.php`

- [ ] **Step 1: `api/auth.php` を作成する**

```php
<?php
// api/auth.php
require_once __DIR__ . '/_db.php';
require_once __DIR__ . '/_auth_check.php';

header('Content-Type: application/json; charset=utf-8');

if (session_status() === PHP_SESSION_NONE) {
    session_start();
}

$action = $_GET['action'] ?? '';
$method = $_SERVER['REQUEST_METHOD'];

if ($action === 'register' && $method === 'POST') {
    $body = json_decode(file_get_contents('php://input'), true);
    $username = trim($body['username'] ?? '');
    $password = $body['password'] ?? '';

    if ($username === '' || strlen($password) < 4) {
        json_response(['error' => 'ユーザー名とパスワード（4文字以上）を入力してください'], 400);
    }

    $pdo = get_db();
    $stmt = $pdo->prepare('SELECT id FROM users WHERE username = ?');
    $stmt->execute([$username]);
    if ($stmt->fetch()) {
        json_response(['error' => 'そのユーザー名は既に使われています'], 409);
    }

    $hash = password_hash($password, PASSWORD_DEFAULT);
    $now = date('Y-m-d H:i:s');
    $stmt = $pdo->prepare('INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)');
    $stmt->execute([$username, $hash, $now]);
    $user_id = (int)$pdo->lastInsertId();

    session_regenerate_id(true);
    $_SESSION['user_id'] = $user_id;
    $_SESSION['username'] = $username;
    json_response(['id' => $user_id, 'username' => $username]);
}

if ($action === 'login' && $method === 'POST') {
    $body = json_decode(file_get_contents('php://input'), true);
    $username = trim($body['username'] ?? '');
    $password = $body['password'] ?? '';

    $pdo = get_db();
    $stmt = $pdo->prepare('SELECT id, password_hash FROM users WHERE username = ?');
    $stmt->execute([$username]);
    $user = $stmt->fetch(PDO::FETCH_ASSOC);

    if (!$user || !password_verify($password, $user['password_hash'])) {
        json_response(['error' => 'ユーザー名またはパスワードが正しくありません'], 401);
    }

    session_regenerate_id(true);
    $_SESSION['user_id'] = (int)$user['id'];
    $_SESSION['username'] = $username;
    json_response(['id' => (int)$user['id'], 'username' => $username]);
}

if ($action === 'logout' && $method === 'POST') {
    $_SESSION = [];
    session_destroy();
    json_response(['ok' => true]);
}

if ($action === 'me' && $method === 'GET') {
    $user_id = require_auth();
    json_response(['id' => $user_id, 'username' => $_SESSION['username']]);
}

json_response(['error' => 'Not Found'], 404);
```

- [ ] **Step 2: ブラウザまたはcurlで動作確認する**

```bash
# 登録
curl -s -c /tmp/cookies.txt -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"test1234"}' \
  http://localhost/api/auth.php?action=register

# ログイン
curl -s -c /tmp/cookies.txt -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"test1234"}' \
  http://localhost/api/auth.php?action=login

# me
curl -s -b /tmp/cookies.txt http://localhost/api/auth.php?action=me
```

期待出力（me）: `{"id":1,"username":"testuser"}`

- [ ] **Step 3: コミットする**

```bash
git add api/auth.php
git commit -m "feat: add auth API (register, login, logout, me)"
```

---

## Task 3: 日別記録API（daily.php）

**Files:**
- Create: `api/daily.php`

- [ ] **Step 1: `api/daily.php` を作成する**

```php
<?php
// api/daily.php
require_once __DIR__ . '/_db.php';
require_once __DIR__ . '/_auth_check.php';

header('Content-Type: application/json; charset=utf-8');

$user_id = require_auth();
$method = $_SERVER['REQUEST_METHOD'];
$pdo = get_db();

if ($method === 'GET') {
    $date = $_GET['date'] ?? get_today_date();
    if (!preg_match('/^\d{4}-\d{2}-\d{2}$/', $date)) {
        json_response(['error' => '日付の形式が正しくありません'], 400);
    }

    $stmt = $pdo->prepare(
        'SELECT date, intake_kcal, exercise_kcal, snack_kcal, memo
         FROM daily_records WHERE user_id = ? AND date = ?'
    );
    $stmt->execute([$user_id, $date]);
    $record = $stmt->fetch(PDO::FETCH_ASSOC);

    // その日の基準カロリー設定を取得
    $s = $pdo->prepare(
        'SELECT base_intake_kcal, base_exercise_kcal FROM calorie_settings
         WHERE user_id = ? AND start_date <= ? AND (end_date IS NULL OR end_date >= ?)
         LIMIT 1'
    );
    $s->execute([$user_id, $date, $date]);
    $setting = $s->fetch(PDO::FETCH_ASSOC);

    json_response([
        'date' => $date,
        'today' => get_today_date(),
        'record' => $record ?: null,
        'setting' => $setting ?: null,
    ]);
}

if ($method === 'POST') {
    $body = json_decode(file_get_contents('php://input'), true);
    $date = $body['date'] ?? get_today_date();

    if (!preg_match('/^\d{4}-\d{2}-\d{2}$/', $date)) {
        json_response(['error' => '日付の形式が正しくありません'], 400);
    }

    $fields = ['intake_kcal', 'exercise_kcal', 'snack_kcal'];
    foreach ($fields as $field) {
        if (isset($body[$field]) && (!is_int($body[$field]) || $body[$field] < 0)) {
            json_response(['error' => $field . 'は0以上の整数で入力してください'], 400);
        }
    }

    $intake   = isset($body['intake_kcal'])   ? (int)$body['intake_kcal']   : null;
    $exercise = isset($body['exercise_kcal']) ? (int)$body['exercise_kcal'] : null;
    $snack    = isset($body['snack_kcal'])    ? (int)$body['snack_kcal']    : null;
    $memo     = isset($body['memo'])          ? (string)$body['memo']       : null;

    $stmt = $pdo->prepare(
        'INSERT INTO daily_records (user_id, date, intake_kcal, exercise_kcal, snack_kcal, memo)
         VALUES (?, ?, ?, ?, ?, ?)
         ON CONFLICT(user_id, date) DO UPDATE SET
           intake_kcal   = excluded.intake_kcal,
           exercise_kcal = excluded.exercise_kcal,
           snack_kcal    = excluded.snack_kcal,
           memo          = excluded.memo'
    );
    $stmt->execute([$user_id, $date, $intake, $exercise, $snack, $memo]);

    json_response(['ok' => true, 'date' => $date]);
}

json_response(['error' => 'Method Not Allowed'], 405);
```

- [ ] **Step 2: 動作確認する**

```bash
# 今日の記録を保存
curl -s -b /tmp/cookies.txt -X POST \
  -H "Content-Type: application/json" \
  -d '{"intake_kcal":1600,"exercise_kcal":300,"snack_kcal":100,"memo":"テスト"}' \
  http://localhost/api/daily.php

# 今日の記録を取得
curl -s -b /tmp/cookies.txt "http://localhost/api/daily.php?date=$(date +%Y-%m-%d)"
```

期待出力: `{"date":"YYYY-MM-DD","today":"YYYY-MM-DD","record":{...},"setting":null}`

- [ ] **Step 3: コミットする**

```bash
git add api/daily.php
git commit -m "feat: add daily records API (GET/POST with upsert)"
```

---

## Task 4: イベントAPI（events.php）

**Files:**
- Create: `api/events.php`

- [ ] **Step 1: `api/events.php` を作成する**

```php
<?php
// api/events.php
require_once __DIR__ . '/_db.php';
require_once __DIR__ . '/_auth_check.php';

header('Content-Type: application/json; charset=utf-8');

$user_id = require_auth();
$method = $_SERVER['REQUEST_METHOD'];
$pdo = get_db();

if ($method === 'GET') {
    $date = $_GET['date'] ?? get_today_date();
    if (!preg_match('/^\d{4}-\d{2}-\d{2}$/', $date)) {
        json_response(['error' => '日付の形式が正しくありません'], 400);
    }
    $stmt = $pdo->prepare(
        'SELECT id, recorded_at, event_type FROM daily_events
         WHERE user_id = ? AND date = ? ORDER BY recorded_at ASC'
    );
    $stmt->execute([$user_id, $date]);
    json_response($stmt->fetchAll(PDO::FETCH_ASSOC));
}

if ($method === 'POST') {
    $body = json_decode(file_get_contents('php://input'), true);
    $event_type = $body['event_type'] ?? '';

    if (!in_array($event_type, ['excretion', 'weigh_in'], true)) {
        json_response(['error' => 'event_typeはexcretionまたはweigh_inを指定してください'], 400);
    }

    $now = date('Y-m-d H:i:s');
    $date = get_today_date();

    $stmt = $pdo->prepare(
        'INSERT INTO daily_events (user_id, recorded_at, event_type, date) VALUES (?, ?, ?, ?)'
    );
    $stmt->execute([$user_id, $now, $event_type, $date]);
    $id = (int)$pdo->lastInsertId();

    json_response(['id' => $id, 'recorded_at' => $now, 'event_type' => $event_type, 'date' => $date]);
}

if ($method === 'DELETE') {
    $id = (int)($_GET['id'] ?? 0);
    if ($id <= 0) {
        json_response(['error' => 'idが必要です'], 400);
    }
    // 自分のレコードのみ削除可能
    $stmt = $pdo->prepare('DELETE FROM daily_events WHERE id = ? AND user_id = ?');
    $stmt->execute([$id, $user_id]);
    json_response(['ok' => true]);
}

json_response(['error' => 'Method Not Allowed'], 405);
```

- [ ] **Step 2: 動作確認する**

```bash
# 排泄を記録
curl -s -b /tmp/cookies.txt -X POST \
  -H "Content-Type: application/json" \
  -d '{"event_type":"excretion"}' \
  http://localhost/api/events.php

# 今日のイベント一覧
curl -s -b /tmp/cookies.txt "http://localhost/api/events.php?date=$(date +%Y-%m-%d)"
```

期待出力: `[{"id":1,"recorded_at":"2026-04-05 10:00:00","event_type":"excretion"}]`

- [ ] **Step 3: コミットする**

```bash
git add api/events.php
git commit -m "feat: add events API (excretion/weigh_in GET/POST/DELETE)"
```

---

## Task 5: 設定API（settings.php）

**Files:**
- Create: `api/settings.php`

- [ ] **Step 1: `api/settings.php` を作成する**

```php
<?php
// api/settings.php
require_once __DIR__ . '/_db.php';
require_once __DIR__ . '/_auth_check.php';

header('Content-Type: application/json; charset=utf-8');

$user_id = require_auth();
$method = $_SERVER['REQUEST_METHOD'];
$pdo = get_db();

// 期間重複チェック（自分自身のIDを除く）
function check_overlap(PDO $pdo, int $user_id, string $start, ?string $end, ?int $exclude_id = null): bool {
    $sql = 'SELECT COUNT(*) FROM calorie_settings
            WHERE user_id = :uid
              AND (:end IS NULL OR start_date <= :end)
              AND (end_date IS NULL OR end_date >= :start)';
    if ($exclude_id !== null) {
        $sql .= ' AND id != :eid';
    }
    $stmt = $pdo->prepare($sql);
    $params = [':uid' => $user_id, ':start' => $start, ':end' => $end];
    if ($exclude_id !== null) {
        $params[':eid'] = $exclude_id;
    }
    $stmt->execute($params);
    return (int)$stmt->fetchColumn() > 0;
}

if ($method === 'GET') {
    $stmt = $pdo->prepare(
        'SELECT id, start_date, end_date, base_intake_kcal, base_exercise_kcal
         FROM calorie_settings WHERE user_id = ? ORDER BY start_date DESC'
    );
    $stmt->execute([$user_id]);
    json_response($stmt->fetchAll(PDO::FETCH_ASSOC));
}

if ($method === 'POST') {
    $body = json_decode(file_get_contents('php://input'), true);
    $id            = isset($body['id']) ? (int)$body['id'] : null;
    $start_date    = $body['start_date'] ?? '';
    $end_date      = $body['end_date'] ?? null;
    $base_intake   = $body['base_intake_kcal'] ?? null;
    $base_exercise = $body['base_exercise_kcal'] ?? null;

    if (!preg_match('/^\d{4}-\d{2}-\d{2}$/', $start_date)) {
        json_response(['error' => 'start_dateの形式が正しくありません'], 400);
    }
    if ($end_date !== null && !preg_match('/^\d{4}-\d{2}-\d{2}$/', $end_date)) {
        json_response(['error' => 'end_dateの形式が正しくありません'], 400);
    }
    if (!is_int($base_intake) || $base_intake < 0 || !is_int($base_exercise) || $base_exercise < 0) {
        json_response(['error' => '基準カロリーは0以上の整数で入力してください'], 400);
    }

    if (check_overlap($pdo, $user_id, $start_date, $end_date, $id)) {
        json_response(['error' => '期間が重複しています'], 409);
    }

    if ($id === null) {
        $stmt = $pdo->prepare(
            'INSERT INTO calorie_settings (user_id, start_date, end_date, base_intake_kcal, base_exercise_kcal)
             VALUES (?, ?, ?, ?, ?)'
        );
        $stmt->execute([$user_id, $start_date, $end_date, $base_intake, $base_exercise]);
        json_response(['id' => (int)$pdo->lastInsertId(), 'ok' => true]);
    } else {
        // 更新：自分のレコードのみ
        $stmt = $pdo->prepare(
            'UPDATE calorie_settings
             SET start_date = ?, end_date = ?, base_intake_kcal = ?, base_exercise_kcal = ?
             WHERE id = ? AND user_id = ?'
        );
        $stmt->execute([$start_date, $end_date, $base_intake, $base_exercise, $id, $user_id]);
        json_response(['ok' => true]);
    }
}

if ($method === 'DELETE') {
    $id = (int)($_GET['id'] ?? 0);
    if ($id <= 0) {
        json_response(['error' => 'idが必要です'], 400);
    }
    $stmt = $pdo->prepare('DELETE FROM calorie_settings WHERE id = ? AND user_id = ?');
    $stmt->execute([$id, $user_id]);
    json_response(['ok' => true]);
}

json_response(['error' => 'Method Not Allowed'], 405);
```

- [ ] **Step 2: 動作確認する**

```bash
# 設定を追加
curl -s -b /tmp/cookies.txt -X POST \
  -H "Content-Type: application/json" \
  -d '{"start_date":"2026-01-01","end_date":null,"base_intake_kcal":1500,"base_exercise_kcal":300}' \
  http://localhost/api/settings.php

# 設定一覧を取得
curl -s -b /tmp/cookies.txt http://localhost/api/settings.php
```

期待出力: `[{"id":1,"start_date":"2026-01-01","end_date":null,"base_intake_kcal":1500,"base_exercise_kcal":300}]`

- [ ] **Step 3: コミットする**

```bash
git add api/settings.php
git commit -m "feat: add calorie settings API with period overlap validation"
```

---

## Task 6: 月間集計API（monthly.php）

**Files:**
- Create: `api/monthly.php`

- [ ] **Step 1: `api/monthly.php` を作成する**

```php
<?php
// api/monthly.php
require_once __DIR__ . '/_db.php';
require_once __DIR__ . '/_auth_check.php';

header('Content-Type: application/json; charset=utf-8');

$user_id = require_auth();
$pdo = get_db();

$year  = (int)($_GET['year']  ?? date('Y'));
$month = (int)($_GET['month'] ?? date('n'));

if ($year < 2000 || $year > 2100 || $month < 1 || $month > 12) {
    json_response(['error' => '年月の値が正しくありません'], 400);
}

$from = sprintf('%04d-%02d-01', $year, $month);
$to   = date('Y-m-t', strtotime($from)); // その月の最終日

// 記録を取得
$stmt = $pdo->prepare(
    'SELECT dr.date, dr.intake_kcal, dr.exercise_kcal, dr.snack_kcal,
            cs.base_intake_kcal, cs.base_exercise_kcal
     FROM daily_records dr
     LEFT JOIN calorie_settings cs
       ON cs.user_id = dr.user_id
      AND cs.start_date <= dr.date
      AND (cs.end_date IS NULL OR cs.end_date >= dr.date)
     WHERE dr.user_id = ? AND dr.date BETWEEN ? AND ?'
);
$stmt->execute([$user_id, $from, $to]);
$rows = $stmt->fetchAll(PDO::FETCH_ASSOC);

$total_diff  = 0;
$total_snack = 0;
$days = 0;

foreach ($rows as $row) {
    $base_in  = (int)($row['base_intake_kcal']   ?? 0);
    $base_ex  = (int)($row['base_exercise_kcal'] ?? 0);
    $intake   = (int)($row['intake_kcal']   ?? 0);
    $exercise = (int)($row['exercise_kcal'] ?? 0);
    $snack    = (int)($row['snack_kcal']    ?? 0);

    // 差分 = 摂取 - 基準摂取 - 運動 + 基準消費
    $diff = $intake - $base_in - $exercise + $base_ex;
    $total_diff  += $diff;
    $total_snack += $snack;
    $days++;
}

json_response([
    'year'           => $year,
    'month'          => $month,
    'total_diff_kcal' => $total_diff,
    'avg_snack_kcal'  => $days > 0 ? (int)round($total_snack / $days) : 0,
    'days_recorded'   => $days,
]);
```

- [ ] **Step 2: 動作確認する**

```bash
curl -s -b /tmp/cookies.txt "http://localhost/api/monthly.php?year=2026&month=4"
```

期待出力: `{"year":2026,"month":4,"total_diff_kcal":100,"avg_snack_kcal":100,"days_recorded":1}`

- [ ] **Step 3: コミットする**

```bash
git add api/monthly.php
git commit -m "feat: add monthly summary API"
```

---

## Task 7: フロントエンド HTML骨格（index.html）

**Files:**
- Create: `index.html`

- [ ] **Step 1: `index.html` を作成する**

```html
<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>カロリー計算機</title>
  <script src="https://unpkg.com/vue@3/dist/vue.global.prod.js"></script>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: sans-serif; background: #f5f5f5; color: #333; }
    #app { max-width: 600px; margin: 0 auto; padding: 16px; }
    .card { background: #fff; border-radius: 8px; padding: 16px; margin-bottom: 16px; box-shadow: 0 1px 4px rgba(0,0,0,0.1); }
    .nav { display: flex; gap: 8px; margin-bottom: 16px; flex-wrap: wrap; }
    .nav button { flex: 1; padding: 8px; border: none; border-radius: 4px; background: #e0e0e0; cursor: pointer; }
    .nav button.active { background: #4caf50; color: #fff; }
    input, textarea { width: 100%; padding: 8px; margin: 4px 0 12px; border: 1px solid #ccc; border-radius: 4px; font-size: 16px; }
    button.primary { background: #4caf50; color: #fff; border: none; border-radius: 4px; padding: 10px 16px; cursor: pointer; font-size: 16px; }
    button.secondary { background: #2196f3; color: #fff; border: none; border-radius: 4px; padding: 10px 16px; cursor: pointer; font-size: 16px; }
    button.danger { background: #f44336; color: #fff; border: none; border-radius: 4px; padding: 8px 12px; cursor: pointer; }
    .error { background: #ffebee; color: #c62828; padding: 10px; border-radius: 4px; margin-bottom: 12px; }
    .diff-plus { color: #f44336; font-weight: bold; }
    .diff-minus { color: #4caf50; font-weight: bold; }
    label { font-size: 14px; color: #555; }
    .row { display: flex; gap: 8px; align-items: center; }
    .event-list { list-style: none; }
    .event-list li { display: flex; justify-content: space-between; align-items: center; padding: 4px 0; border-bottom: 1px solid #eee; font-size: 14px; }
    table { width: 100%; border-collapse: collapse; font-size: 14px; }
    th, td { padding: 8px; text-align: left; border-bottom: 1px solid #eee; }
    th { background: #f9f9f9; }
  </style>
</head>
<body>
  <div id="app">
    <div v-if="error" class="error">{{ error }}</div>
    <div v-if="!user">
      <!-- ログイン・登録画面はapp.jsで描画 -->
      <login-view @login="onLogin" @register="onRegister"></login-view>
    </div>
    <div v-else>
      <div class="card">
        <div class="row" style="justify-content:space-between">
          <span>{{ user.username }} さん</span>
          <button class="danger" @click="logout">ログアウト</button>
        </div>
      </div>
      <nav class="nav">
        <button :class="{active: page==='today'}"   @click="page='today'">今日の記録</button>
        <button :class="{active: page==='list'}"    @click="page='list'">日別一覧</button>
        <button :class="{active: page==='monthly'}" @click="page='monthly'">月間サマリー</button>
        <button :class="{active: page==='settings'}"@click="page='settings'">設定</button>
      </nav>
      <today-view   v-if="page==='today'"    :user="user" @error="showError"></today-view>
      <list-view    v-if="page==='list'"     :user="user" @error="showError"></list-view>
      <monthly-view v-if="page==='monthly'"  :user="user" @error="showError"></monthly-view>
      <settings-view v-if="page==='settings'" :user="user" @error="showError"></settings-view>
    </div>
  </div>
  <script src="js/app.js"></script>
</body>
</html>
```

- [ ] **Step 2: コミットする**

```bash
git add index.html
git commit -m "feat: add HTML skeleton with Vue.js CDN"
```

---

## Task 8: Vue.js アプリ本体（app.js）— 認証・共通部

**Files:**
- Create: `js/app.js`

- [ ] **Step 1: `js/app.js` を作成する（認証部分と共通ユーティリティ）**

```js
// js/app.js
const { createApp, ref, reactive, computed, onMounted } = Vue;

// 共通API呼び出しユーティリティ
async function api(path, options = {}) {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    ...options,
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'エラーが発生しました');
  return data;
}

// ログイン・登録コンポーネント
const LoginView = {
  emits: ['login', 'register'],
  setup(props, { emit }) {
    const mode = ref('login'); // 'login' or 'register'
    const username = ref('');
    const password = ref('');
    const localError = ref('');

    async function submit() {
      localError.value = '';
      try {
        const action = mode.value === 'login' ? 'login' : 'register';
        const data = await api(`api/auth.php?action=${action}`, {
          method: 'POST',
          body: JSON.stringify({ username: username.value, password: password.value }),
        });
        emit(action, data);
      } catch (e) {
        localError.value = e.message;
      }
    }

    return { mode, username, password, localError, submit };
  },
  template: `
    <div class="card">
      <h2 style="margin-bottom:16px">{{ mode === 'login' ? 'ログイン' : '新規登録' }}</h2>
      <div v-if="localError" class="error">{{ localError }}</div>
      <label>ユーザー名</label>
      <input v-model="username" type="text" placeholder="ユーザー名">
      <label>パスワード</label>
      <input v-model="password" type="password" placeholder="パスワード（4文字以上）">
      <button class="primary" @click="submit" style="width:100%">
        {{ mode === 'login' ? 'ログイン' : '登録する' }}
      </button>
      <p style="margin-top:12px;font-size:14px;text-align:center">
        <a href="#" @click.prevent="mode = mode==='login'?'register':'login'">
          {{ mode === 'login' ? '新規登録はこちら' : 'ログイン画面へ' }}
        </a>
      </p>
    </div>
  `,
};
```

- [ ] **Step 2: コミットする**

```bash
git add js/app.js
git commit -m "feat: add Vue.js app skeleton with login/register component"
```

---

## Task 9: Vue.js — 今日の記録コンポーネント

**Files:**
- Modify: `js/app.js`

- [ ] **Step 1: `TodayView` コンポーネントを `js/app.js` に追記する**

`LoginView` の定義の後に追加：

```js
// 今日の記録コンポーネント
const TodayView = {
  props: ['user'],
  emits: ['error'],
  setup(props, { emit }) {
    const record = reactive({ intake_kcal: '', exercise_kcal: '', snack_kcal: '', memo: '' });
    const setting = ref(null);
    const events = ref([]);
    const today = ref('');
    const saving = ref(false);

    const diff = computed(() => {
      if (!setting.value) return null;
      const intake   = parseInt(record.intake_kcal)   || 0;
      const exercise = parseInt(record.exercise_kcal) || 0;
      return intake - setting.value.base_intake_kcal - exercise + setting.value.base_exercise_kcal;
    });

    async function load() {
      try {
        const data = await api('api/daily.php');
        today.value = data.today;
        if (data.record) {
          record.intake_kcal   = data.record.intake_kcal   ?? '';
          record.exercise_kcal = data.record.exercise_kcal ?? '';
          record.snack_kcal    = data.record.snack_kcal    ?? '';
          record.memo          = data.record.memo          ?? '';
        }
        setting.value = data.setting;
        const ev = await api(`api/events.php?date=${data.today}`);
        events.value = ev;
      } catch (e) {
        emit('error', e.message);
      }
    }

    async function save() {
      saving.value = true;
      try {
        await api('api/daily.php', {
          method: 'POST',
          body: JSON.stringify({
            intake_kcal:   record.intake_kcal   !== '' ? parseInt(record.intake_kcal)   : null,
            exercise_kcal: record.exercise_kcal !== '' ? parseInt(record.exercise_kcal) : null,
            snack_kcal:    record.snack_kcal    !== '' ? parseInt(record.snack_kcal)    : null,
            memo:          record.memo,
          }),
        });
      } catch (e) {
        emit('error', e.message);
      } finally {
        saving.value = false;
      }
    }

    async function recordEvent(type) {
      try {
        const ev = await api('api/events.php', {
          method: 'POST',
          body: JSON.stringify({ event_type: type }),
        });
        events.value.push(ev);
      } catch (e) {
        emit('error', e.message);
      }
    }

    async function deleteEvent(id) {
      try {
        await api(`api/events.php?id=${id}`, { method: 'DELETE' });
        events.value = events.value.filter(e => e.id !== id);
      } catch (e) {
        emit('error', e.message);
      }
    }

    const eventLabel = (type) => type === 'excretion' ? '排泄' : '体重計測';

    onMounted(load);
    return { record, setting, events, today, saving, diff, save, recordEvent, deleteEvent, eventLabel };
  },
  template: `
    <div>
      <div class="card">
        <h3 style="margin-bottom:12px">今日の記録（{{ today }}）</h3>
        <div v-if="setting">
          <p style="font-size:13px;color:#666;margin-bottom:12px">
            基準: 摂取 {{ setting.base_intake_kcal }} kcal / 消費 {{ setting.base_exercise_kcal }} kcal
          </p>
        </div>
        <div v-else style="font-size:13px;color:#f44336;margin-bottom:12px">
          ⚠ 設定から基準カロリーを登録してください
        </div>
        <label>摂取カロリー (kcal)</label>
        <input v-model="record.intake_kcal" type="number" min="0" placeholder="例: 1600">
        <label>運動消費カロリー (kcal)</label>
        <input v-model="record.exercise_kcal" type="number" min="0" placeholder="例: 300">
        <label>お菓子カロリー (kcal)</label>
        <input v-model="record.snack_kcal" type="number" min="0" placeholder="例: 100">
        <label>メモ</label>
        <textarea v-model="record.memo" rows="2" placeholder="自由メモ"></textarea>
        <div v-if="diff !== null" style="margin-bottom:12px;font-size:18px">
          差分:
          <span :class="diff > 0 ? 'diff-plus' : 'diff-minus'">
            {{ diff > 0 ? '+' : '' }}{{ diff }} kcal
          </span>
        </div>
        <button class="primary" @click="save" :disabled="saving">
          {{ saving ? '保存中...' : '保存' }}
        </button>
      </div>
      <div class="card">
        <h3 style="margin-bottom:12px">記録ボタン</h3>
        <div class="row" style="gap:12px;margin-bottom:16px">
          <button class="secondary" @click="recordEvent('excretion')" style="flex:1">排泄 🚽</button>
          <button class="secondary" @click="recordEvent('weigh_in')" style="flex:1">体重計測 ⚖️</button>
        </div>
        <ul class="event-list">
          <li v-for="ev in events" :key="ev.id">
            <span>{{ ev.recorded_at.slice(11,16) }} {{ eventLabel(ev.event_type) }}</span>
            <button class="danger" @click="deleteEvent(ev.id)" style="padding:2px 8px;font-size:12px">削除</button>
          </li>
        </ul>
        <p v-if="events.length === 0" style="font-size:13px;color:#999">まだ記録がありません</p>
      </div>
    </div>
  `,
};
```

- [ ] **Step 2: コミットする**

```bash
git add js/app.js
git commit -m "feat: add TodayView component with diff calculation and event buttons"
```

---

## Task 10: Vue.js — 日別一覧・月間サマリー・設定コンポーネント

**Files:**
- Modify: `js/app.js`

- [ ] **Step 1: `ListView` を `js/app.js` に追記する**

`TodayView` の定義の後に追加：

```js
// 日別一覧コンポーネント
const ListView = {
  props: ['user'],
  emits: ['error'],
  setup(props, { emit }) {
    const records = ref([]);
    const editing = ref(null); // 編集中の日付
    const editData = reactive({ intake_kcal: '', exercise_kcal: '', snack_kcal: '', memo: '' });

    async function load() {
      // 直近30日分を取得
      const today = new Date();
      const dates = [];
      for (let i = 0; i < 30; i++) {
        const d = new Date(today);
        d.setDate(today.getDate() - i);
        dates.push(d.toISOString().slice(0, 10));
      }
      const results = await Promise.all(
        dates.map(date => api(`api/daily.php?date=${date}`).catch(() => null))
      );
      records.value = results
        .filter(r => r && r.record)
        .map(r => ({ ...r.record, setting: r.setting }));
    }

    function startEdit(rec) {
      editing.value = rec.date;
      editData.intake_kcal   = rec.intake_kcal   ?? '';
      editData.exercise_kcal = rec.exercise_kcal ?? '';
      editData.snack_kcal    = rec.snack_kcal    ?? '';
      editData.memo          = rec.memo          ?? '';
    }

    async function saveEdit(date) {
      try {
        await api('api/daily.php', {
          method: 'POST',
          body: JSON.stringify({
            date,
            intake_kcal:   editData.intake_kcal   !== '' ? parseInt(editData.intake_kcal)   : null,
            exercise_kcal: editData.exercise_kcal !== '' ? parseInt(editData.exercise_kcal) : null,
            snack_kcal:    editData.snack_kcal    !== '' ? parseInt(editData.snack_kcal)    : null,
            memo:          editData.memo,
          }),
        });
        editing.value = null;
        await load();
      } catch (e) {
        emit('error', e.message);
      }
    }

    function calcDiff(rec) {
      if (!rec.setting) return null;
      const intake   = rec.intake_kcal   ?? 0;
      const exercise = rec.exercise_kcal ?? 0;
      return intake - rec.setting.base_intake_kcal - exercise + rec.setting.base_exercise_kcal;
    }

    onMounted(load);
    return { records, editing, editData, startEdit, saveEdit, calcDiff };
  },
  template: `
    <div class="card">
      <h3 style="margin-bottom:12px">日別一覧（直近30日）</h3>
      <p v-if="records.length === 0" style="color:#999;font-size:14px">記録がありません</p>
      <div v-for="rec in records" :key="rec.date" style="margin-bottom:8px;border-bottom:1px solid #eee;padding-bottom:8px">
        <div class="row" style="justify-content:space-between">
          <strong>{{ rec.date }}</strong>
          <span v-if="calcDiff(rec) !== null" :class="calcDiff(rec) > 0 ? 'diff-plus' : 'diff-minus'">
            {{ calcDiff(rec) > 0 ? '+' : '' }}{{ calcDiff(rec) }} kcal
          </span>
          <button class="secondary" @click="startEdit(rec)" style="padding:4px 8px;font-size:12px">編集</button>
        </div>
        <div v-if="editing !== rec.date" style="font-size:13px;color:#555;margin-top:4px">
          摂取: {{ rec.intake_kcal ?? '-' }} / 運動: {{ rec.exercise_kcal ?? '-' }} / お菓子: {{ rec.snack_kcal ?? '-' }}
          <span v-if="rec.memo"> | {{ rec.memo }}</span>
        </div>
        <div v-else style="margin-top:8px">
          <input v-model="editData.intake_kcal"   type="number" min="0" placeholder="摂取kcal">
          <input v-model="editData.exercise_kcal" type="number" min="0" placeholder="運動kcal">
          <input v-model="editData.snack_kcal"    type="number" min="0" placeholder="お菓子kcal">
          <input v-model="editData.memo" type="text" placeholder="メモ">
          <div class="row" style="gap:8px">
            <button class="primary" @click="saveEdit(rec.date)">保存</button>
            <button @click="editing=null" style="padding:8px 12px;border:1px solid #ccc;border-radius:4px;cursor:pointer">キャンセル</button>
          </div>
        </div>
      </div>
    </div>
  `,
};
```

- [ ] **Step 2: `MonthlyView` を追記する**

```js
// 月間サマリーコンポーネント
const MonthlyView = {
  props: ['user'],
  emits: ['error'],
  setup(props, { emit }) {
    const summary = ref(null);
    const year  = ref(new Date().getFullYear());
    const month = ref(new Date().getMonth() + 1);

    async function load() {
      try {
        summary.value = await api(`api/monthly.php?year=${year.value}&month=${month.value}`);
      } catch (e) {
        emit('error', e.message);
      }
    }

    function prevMonth() {
      if (month.value === 1) { year.value--; month.value = 12; }
      else month.value--;
      load();
    }

    function nextMonth() {
      if (month.value === 12) { year.value++; month.value = 1; }
      else month.value++;
      load();
    }

    onMounted(load);
    return { summary, year, month, prevMonth, nextMonth };
  },
  template: `
    <div class="card">
      <div class="row" style="justify-content:space-between;margin-bottom:16px">
        <button @click="prevMonth" style="padding:6px 12px;border:1px solid #ccc;border-radius:4px;cursor:pointer">◀</button>
        <h3>{{ year }}年 {{ month }}月</h3>
        <button @click="nextMonth" style="padding:6px 12px;border:1px solid #ccc;border-radius:4px;cursor:pointer">▶</button>
      </div>
      <div v-if="summary">
        <table>
          <tr><th>記録日数</th><td>{{ summary.days_recorded }} 日</td></tr>
          <tr>
            <th>累計差分</th>
            <td :class="summary.total_diff_kcal > 0 ? 'diff-plus' : 'diff-minus'">
              {{ summary.total_diff_kcal > 0 ? '+' : '' }}{{ summary.total_diff_kcal }} kcal
            </td>
          </tr>
          <tr><th>平均お菓子</th><td>{{ summary.avg_snack_kcal }} kcal/日</td></tr>
        </table>
        <p v-if="summary.days_recorded === 0" style="margin-top:12px;color:#999;font-size:14px">この月の記録はありません</p>
      </div>
    </div>
  `,
};
```

- [ ] **Step 3: `SettingsView` を追記する**

```js
// 設定コンポーネント（基準カロリー期間管理）
const SettingsView = {
  props: ['user'],
  emits: ['error'],
  setup(props, { emit }) {
    const settings = ref([]);
    const form = reactive({ id: null, start_date: '', end_date: '', base_intake_kcal: '', base_exercise_kcal: '' });
    const editing = ref(false);

    async function load() {
      try {
        settings.value = await api('api/settings.php');
      } catch (e) {
        emit('error', e.message);
      }
    }

    function startNew() {
      form.id = null; form.start_date = ''; form.end_date = '';
      form.base_intake_kcal = ''; form.base_exercise_kcal = '';
      editing.value = true;
    }

    function startEdit(s) {
      form.id = s.id; form.start_date = s.start_date; form.end_date = s.end_date ?? '';
      form.base_intake_kcal = s.base_intake_kcal; form.base_exercise_kcal = s.base_exercise_kcal;
      editing.value = true;
    }

    async function save() {
      try {
        await api('api/settings.php', {
          method: 'POST',
          body: JSON.stringify({
            id: form.id || undefined,
            start_date: form.start_date,
            end_date: form.end_date || null,
            base_intake_kcal:   parseInt(form.base_intake_kcal),
            base_exercise_kcal: parseInt(form.base_exercise_kcal),
          }),
        });
        editing.value = false;
        await load();
      } catch (e) {
        emit('error', e.message);
      }
    }

    async function remove(id) {
      if (!confirm('この設定を削除しますか？')) return;
      try {
        await api(`api/settings.php?id=${id}`, { method: 'DELETE' });
        await load();
      } catch (e) {
        emit('error', e.message);
      }
    }

    onMounted(load);
    return { settings, form, editing, startNew, startEdit, save, remove };
  },
  template: `
    <div>
      <div class="card">
        <div class="row" style="justify-content:space-between;margin-bottom:12px">
          <h3>基準カロリー設定</h3>
          <button class="primary" @click="startNew" style="padding:6px 12px">＋ 追加</button>
        </div>
        <div v-if="editing" style="margin-bottom:16px;padding:12px;background:#f9f9f9;border-radius:4px">
          <label>開始日</label>
          <input v-model="form.start_date" type="date">
          <label>終了日（空欄 = 無期限）</label>
          <input v-model="form.end_date" type="date">
          <label>基準摂取カロリー (kcal)</label>
          <input v-model="form.base_intake_kcal" type="number" min="0" placeholder="例: 1500">
          <label>基準消費カロリー (kcal)</label>
          <input v-model="form.base_exercise_kcal" type="number" min="0" placeholder="例: 300">
          <div class="row" style="gap:8px">
            <button class="primary" @click="save">保存</button>
            <button @click="editing=false" style="padding:8px 12px;border:1px solid #ccc;border-radius:4px;cursor:pointer">キャンセル</button>
          </div>
        </div>
        <p v-if="settings.length === 0" style="color:#999;font-size:14px">設定がありません</p>
        <table v-else>
          <thead><tr><th>期間</th><th>摂取</th><th>消費</th><th></th></tr></thead>
          <tbody>
            <tr v-for="s in settings" :key="s.id">
              <td>{{ s.start_date }} 〜 {{ s.end_date ?? '無期限' }}</td>
              <td>{{ s.base_intake_kcal }}</td>
              <td>{{ s.base_exercise_kcal }}</td>
              <td>
                <button class="secondary" @click="startEdit(s)" style="padding:4px 8px;font-size:12px;margin-right:4px">編集</button>
                <button class="danger" @click="remove(s.id)" style="padding:4px 8px;font-size:12px">削除</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  `,
};
```

- [ ] **Step 4: コミットする**

```bash
git add js/app.js
git commit -m "feat: add ListView, MonthlyView, SettingsView components"
```

---

## Task 11: Vue.js アプリのマウント（app.js 完成）

**Files:**
- Modify: `js/app.js`

- [ ] **Step 1: アプリのマウントコードを `js/app.js` の末尾に追記する**

```js
// メインアプリ
const app = createApp({
  components: { LoginView, TodayView, ListView, MonthlyView, SettingsView },
  setup() {
    const user  = ref(null);
    const page  = ref('today');
    const error = ref('');
    let errorTimer = null;

    function showError(msg) {
      error.value = msg;
      clearTimeout(errorTimer);
      errorTimer = setTimeout(() => { error.value = ''; }, 3000);
    }

    async function checkSession() {
      try {
        user.value = await api('api/auth.php?action=me');
      } catch (_) {
        user.value = null;
      }
    }

    function onLogin(data)    { user.value = data; page.value = 'today'; }
    function onRegister(data) { user.value = data; page.value = 'today'; }

    async function logout() {
      await api('api/auth.php?action=logout', { method: 'POST' });
      user.value = null;
    }

    onMounted(checkSession);
    return { user, page, error, showError, onLogin, onRegister, logout };
  },
});
app.mount('#app');
```

- [ ] **Step 2: ブラウザで全画面を一通り操作して動作確認する**

確認項目：
1. ログイン・登録が機能する
2. 今日の記録が保存・差分表示される
3. 排泄・体重計測ボタンが記録される
4. 日別一覧で過去記録が編集できる
5. 月間サマリーが正しく表示される
6. 設定で基準カロリーが追加・編集・削除できる
7. 基準カロリー設定後に今日の記録画面の差分が更新される

- [ ] **Step 3: コミットする**

```bash
git add js/app.js
git commit -m "feat: mount Vue.js app with session check and error display"
```

---

## Task 12: Apache 設定（さくらサーバー対応）

**Files:**
- Create: `.htaccess`

- [ ] **Step 1: ルートに `.htaccess` を作成する**

```apacheconf
# .htaccess
# セッションの安全設定
php_value session.cookie_httponly 1
php_value session.use_strict_mode 1

# SPAのルーティング：全リクエストをindex.htmlへ（APIパスは除く）
RewriteEngine On
RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteCond %{REQUEST_URI} !^/api/
RewriteRule ^ index.html [L]
```

- [ ] **Step 2: コミットする**

```bash
git add .htaccess
git commit -m "feat: add .htaccess for Apache SPA routing and session security"
```

---

## Task 13: さくらサーバーへのデプロイ確認

**Files:** なし（デプロイ手順）

- [ ] **Step 1: ファイルをさくらサーバーへアップロードする**

FTPまたはSFTPで以下をアップロード：
```
index.html
.htaccess
js/app.js
api/_db.php
api/_auth_check.php
api/auth.php
api/daily.php
api/events.php
api/settings.php
api/monthly.php
db/.htaccess
```

- [ ] **Step 2: `db/` ディレクトリの書き込み権限を確認する**

さくらのFTPまたはSSHで：
```bash
chmod 755 db/
```

SQLiteファイルはPHPが初回アクセス時に自動生成されます。

- [ ] **Step 3: ブラウザでアクセスして動作確認する**

本番URLにアクセスし、Task 11 Step 2と同じ確認項目をチェックする。

---

## セルフレビュー結果

**スペック対応確認：**
- ✅ ユーザー登録・ログイン・ログアウト → Task 2
- ✅ 日別カロリー記録（摂取・運動・お菓子・メモ） → Task 3, 9
- ✅ 朝5時ルール → Task 1 (`get_today_date()`)
- ✅ 排泄・体重計測のタイムスタンプ記録 → Task 4, 9
- ✅ 基準カロリー期間設定・重複チェック → Task 5, 10
- ✅ 差分計算（摂取-基準摂取-運動+基準消費） → Task 9
- ✅ 月間累計差分・平均お菓子カロリー → Task 6, 10
- ✅ 日別一覧（過去記録の編集） → Task 10
- ✅ セキュリティ（password_hash, PDO, session_regenerate_id, .htaccess） → Task 1, 2
- ✅ エラーハンドリング（3秒表示） → Task 11
- ✅ さくらサーバー対応 → Task 12, 13
