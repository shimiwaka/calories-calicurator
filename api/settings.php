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
