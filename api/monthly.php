<?php
// api/monthly.php
require_once __DIR__ . '/_db.php';
require_once __DIR__ . '/_auth_check.php';

header('Content-Type: application/json; charset=utf-8');

set_exception_handler(function (Throwable $e) {
    http_response_code(500);
    echo json_encode(['error' => $e->getMessage()], JSON_UNESCAPED_UNICODE);
    exit;
});

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
    'year'            => $year,
    'month'           => $month,
    'total_diff_kcal' => $total_diff,
    'avg_snack_kcal'  => $days > 0 ? (int)round($total_snack / $days) : 0,
    'days_recorded'   => $days,
]);
