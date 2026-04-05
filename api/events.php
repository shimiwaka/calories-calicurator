<?php
// api/events.php
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
        'SELECT id, recorded_at, event_type FROM daily_events
         WHERE user_id = ? AND date = ? ORDER BY recorded_at ASC'
    );
    $stmt->execute([$user_id, $date]);
    $rows = $stmt->fetchAll(PDO::FETCH_ASSOC);
    foreach ($rows as &$row) {
        $row['id'] = (int)$row['id'];
    }
    json_response($rows);
}

if ($method === 'POST') {
    $body = json_decode(file_get_contents('php://input'), true);
    $event_type = $body['event_type'] ?? '';

    if (!in_array($event_type, ['excretion', 'weigh_in'], true)) {
        json_response(['error' => 'event_typeはexcretionまたはweigh_inを指定してください'], 400);
    }

    $now = date('Y-m-d H:i:s');
    // 明示的にdateが渡された場合はそれを使用（過去日付への記録）
    $date = (isset($body['date']) && preg_match('/^\d{4}-\d{2}-\d{2}$/', $body['date']))
        ? $body['date']
        : get_today_date();

    $stmt = $pdo->prepare(
        'INSERT INTO daily_events (user_id, recorded_at, event_type, date) VALUES (?, ?, ?, ?)'
    );
    $stmt->execute([$user_id, $now, $event_type, $date]);
    $id = (int)$pdo->lastInsertId();

    json_response(['id' => $id, 'recorded_at' => $now, 'event_type' => $event_type, 'date' => $date]);
}

if ($method === 'PUT') {
    $body = json_decode(file_get_contents('php://input'), true);
    $id   = (int)($body['id'] ?? 0);
    $time = $body['time'] ?? ''; // "HH:MM" 形式

    if ($id <= 0 || !preg_match('/^\d{2}:\d{2}$/', $time)) {
        json_response(['error' => 'idとtime（HH:MM形式）が必要です'], 400);
    }

    // 現在のイベントの日付を取得して、時刻だけ変更する
    $stmt = $pdo->prepare('SELECT date FROM daily_events WHERE id = ? AND user_id = ?');
    $stmt->execute([$id, $user_id]);
    $ev = $stmt->fetch(PDO::FETCH_ASSOC);
    if (!$ev) {
        json_response(['error' => 'イベントが見つかりません'], 404);
    }

    $recorded_at = $ev['date'] . ' ' . $time . ':00';
    $stmt = $pdo->prepare('UPDATE daily_events SET recorded_at = ? WHERE id = ? AND user_id = ?');
    $stmt->execute([$recorded_at, $id, $user_id]);
    json_response(['ok' => true, 'recorded_at' => $recorded_at]);
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
