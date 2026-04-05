<?php
// api/daily.php
require_once __DIR__ . '/_db.php';
require_once __DIR__ . '/_auth_check.php';

header('Content-Type: application/json; charset=utf-8');

set_exception_handler(function (Throwable $e) {
    http_response_code(500);
    echo json_encode(['error' => $e->getMessage()], JSON_UNESCAPED_UNICODE);
    exit;
});

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

    // PDO/SQLiteは値を文字列で返すため整数にキャスト
    if ($record) {
        foreach (['intake_kcal', 'exercise_kcal', 'snack_kcal'] as $f) {
            $record[$f] = $record[$f] !== null ? (int)$record[$f] : null;
        }
    }
    if ($setting) {
        $setting['base_intake_kcal']   = (int)$setting['base_intake_kcal'];
        $setting['base_exercise_kcal'] = (int)$setting['base_exercise_kcal'];
    }

    json_response([
        'date'    => $date,
        'today'   => get_today_date(),
        'record'  => $record ?: null,
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
        if (isset($body[$field]) && $body[$field] !== null && (!is_int($body[$field]) || $body[$field] < 0)) {
            json_response(['error' => $field . 'は0以上の整数で入力してください'], 400);
        }
    }

    $intake   = isset($body['intake_kcal'])   && $body['intake_kcal']   !== null ? (int)$body['intake_kcal']   : null;
    $exercise = isset($body['exercise_kcal']) && $body['exercise_kcal'] !== null ? (int)$body['exercise_kcal'] : null;
    $snack    = isset($body['snack_kcal'])    && $body['snack_kcal']    !== null ? (int)$body['snack_kcal']    : null;
    $memo     = isset($body['memo'])          ? (string)$body['memo']   : null;

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
