<?php
// api/_auth_check.php
define('SESSION_LIFETIME', 60 * 60 * 24 * 7); // 1週間

function start_session(): void {
    if (session_status() === PHP_SESSION_NONE) {
        ini_set('session.gc_maxlifetime', SESSION_LIFETIME);
        session_set_cookie_params(['lifetime' => SESSION_LIFETIME, 'samesite' => 'Lax']);
        session_start();
    }
}

function require_auth(): int {
    start_session();
    if (empty($_SESSION['user_id'])) {
        http_response_code(401);
        header('Content-Type: application/json; charset=utf-8');
        echo json_encode(['error' => 'Unauthorized']);
        exit;
    }
    return (int)$_SESSION['user_id'];
}

function json_response($data, int $status = 200): void {
    http_response_code($status);
    header('Content-Type: application/json; charset=utf-8');
    echo json_encode($data, JSON_UNESCAPED_UNICODE);
    exit;
}
