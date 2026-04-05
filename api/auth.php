<?php
// api/auth.php
require_once __DIR__ . '/_db.php';
require_once __DIR__ . '/_auth_check.php';

header('Content-Type: application/json; charset=utf-8');

// 未捕捉の例外をJSONで返す
set_exception_handler(function (Throwable $e) {
    http_response_code(500);
    echo json_encode(['error' => $e->getMessage()], JSON_UNESCAPED_UNICODE);
    exit;
});

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
