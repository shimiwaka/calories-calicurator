<?php
// api/_db.php
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
