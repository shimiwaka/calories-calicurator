<?php
// api/api_settings.php  外部API設定の取得・保存
require_once __DIR__ . '/_db.php';
require_once __DIR__ . '/_auth_check.php';

header('Content-Type: application/json; charset=utf-8');

set_exception_handler(function (Throwable $e) {
    http_response_code(500);
    echo json_encode(['error' => $e->getMessage()], JSON_UNESCAPED_UNICODE);
    exit;
});

$user_id = require_auth();
$method  = $_SERVER['REQUEST_METHOD'];
$pdo     = get_db();

if ($method === 'GET') {
    $stmt = $pdo->prepare('SELECT api_url, api_token FROM user_api_settings WHERE user_id = ?');
    $stmt->execute([$user_id]);
    $row = $stmt->fetch(PDO::FETCH_ASSOC);
    json_response($row ?: ['api_url' => '', 'api_token' => '']);
}

if ($method === 'POST') {
    $body      = json_decode(file_get_contents('php://input'), true);
    $api_url   = trim($body['api_url']   ?? '');
    $api_token = trim($body['api_token'] ?? '');

    $stmt = $pdo->prepare(
        'INSERT INTO user_api_settings (user_id, api_url, api_token)
         VALUES (?, ?, ?)
         ON CONFLICT(user_id) DO UPDATE SET api_url = excluded.api_url, api_token = excluded.api_token'
    );
    $stmt->execute([$user_id, $api_url, $api_token]);
    json_response(['ok' => true]);
}

json_response(['error' => 'Method Not Allowed'], 405);
