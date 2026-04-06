// js/app.js
const { createApp, ref, reactive, computed, onMounted, watch } = Vue;

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

// 今日の記録コンポーネント
const TodayView = {
  props: ['user'],
  emits: ['error'],
  setup(props, { emit }) {
    const record = reactive({ intake_kcal: '', exercise_kcal: '', snack_kcal: '', memo: '' });
    const setting = ref(null);
    const events = ref([]);
    const today = ref('');
    const selectedDate = ref('');
    const saving = ref(false);
    const editingEventId = ref(null);
    const editingTime = ref('');

    const diff = computed(() => {
      if (!setting.value) return null;
      const intake   = parseInt(record.intake_kcal)   || 0;
      const exercise = parseInt(record.exercise_kcal) || 0;
      return intake - setting.value.base_intake_kcal - exercise + setting.value.base_exercise_kcal;
    });

    async function loadDate(date) {
      try {
        const data = await api(`api/daily.php?date=${date}`);
        today.value = data.today;
        record.intake_kcal = ''; record.exercise_kcal = ''; record.snack_kcal = ''; record.memo = '';
        if (data.record) {
          record.intake_kcal   = data.record.intake_kcal   ?? '';
          record.exercise_kcal = data.record.exercise_kcal ?? '';
          record.snack_kcal    = data.record.snack_kcal    ?? '';
          record.memo          = data.record.memo          ?? '';
        }
        setting.value = data.setting;
        const ev = await api(`api/events.php?date=${date}`);
        events.value = ev;
        editingEventId.value = null;
      } catch (e) {
        emit('error', e.message);
      }
    }

    async function load() {
      // 初回：今日の日付をAPIから取得して selectedDate にセット
      try {
        const data = await api('api/daily.php');
        selectedDate.value = data.today;
      } catch (e) {
        emit('error', e.message);
      }
    }

    watch(selectedDate, (date) => {
      if (date) loadDate(date);
    });

    async function save() {
      saving.value = true;
      try {
        await api('api/daily.php', {
          method: 'POST',
          body: JSON.stringify({
            date:          selectedDate.value,
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
          body: JSON.stringify({ event_type: type, date: selectedDate.value }),
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

    function startEditEvent(ev) {
      editingEventId.value = ev.id;
      editingTime.value = ev.recorded_at.slice(11, 16); // "HH:MM"
    }

    async function saveEventTime(id) {
      try {
        const res = await api('api/events.php', {
          method: 'PUT',
          body: JSON.stringify({ id, time: editingTime.value }),
        });
        const ev = events.value.find(e => e.id === id);
        if (ev) ev.recorded_at = res.recorded_at;
        editingEventId.value = null;
      } catch (e) {
        emit('error', e.message);
      }
    }

    const eventLabel = (type) => type === 'excretion' ? '🚽' : '⚖️';

    onMounted(load);
    return {
      record, setting, events, today, selectedDate, saving, diff,
      editingEventId, editingTime,
      save, recordEvent, deleteEvent, startEditEvent, saveEventTime, eventLabel,
    };
  },
  template: `
    <div>
      <div class="card">
        <div class="row" style="justify-content:space-between;margin-bottom:12px">
          <h3>記録する日付</h3>
          <input v-model="selectedDate" type="date" style="width:auto;padding:6px;margin:0">
        </div>
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
          <button class="secondary" @click="recordEvent('excretion')" style="flex:1">🚽</button>
          <button class="secondary" @click="recordEvent('weigh_in')" style="flex:1">⚖️</button>
        </div>
        <ul class="event-list">
          <li v-for="ev in events" :key="ev.id">
            <span v-if="editingEventId !== ev.id">{{ ev.recorded_at.slice(11,16) }} {{ eventLabel(ev.event_type) }}</span>
            <span v-else style="display:flex;align-items:center;gap:6px">
              <input v-model="editingTime" type="time" style="width:auto;padding:4px;margin:0;font-size:14px">
              <span style="font-size:13px">{{ eventLabel(ev.event_type) }}</span>
            </span>
            <div style="display:flex;gap:4px">
              <button v-if="editingEventId !== ev.id" class="secondary" @click="startEditEvent(ev)" style="padding:2px 8px;font-size:12px">時刻編集</button>
              <button v-if="editingEventId === ev.id" class="primary" @click="saveEventTime(ev.id)" style="padding:2px 8px;font-size:12px">保存</button>
              <button v-if="editingEventId === ev.id" @click="editingEventId=null" style="padding:2px 8px;font-size:12px;border:1px solid #ccc;border-radius:4px;cursor:pointer;background:#fff">×</button>
              <button class="danger" @click="deleteEvent(ev.id)" style="padding:2px 8px;font-size:12px">削除</button>
            </div>
          </li>
        </ul>
        <p v-if="events.length === 0" style="font-size:13px;color:#999">まだ記録がありません</p>
      </div>
    </div>
  `,
};

// 日別一覧コンポーネント
const ListView = {
  props: ['user'],
  emits: ['error'],
  setup(props, { emit }) {
    const records = ref([]);
    const editing = ref(null);
    const editData = reactive({ intake_kcal: '', exercise_kcal: '', snack_kcal: '', memo: '' });

    async function load() {
      const today = new Date();
      const dates = [];
      for (let i = 0; i < 30; i++) {
        const d = new Date(today);
        d.setDate(today.getDate() - i);
        dates.push(d.toISOString().slice(0, 10));
      }
      const results = await Promise.all(
        dates.map(date => Promise.all([
          api(`api/daily.php?date=${date}`).catch(() => null),
          api(`api/events.php?date=${date}`).catch(() => []),
        ]))
      );
      records.value = results
        .filter(([r]) => r && r.record)
        .map(([r, evs]) => ({ ...r.record, setting: r.setting, events: evs }));
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
        <div v-if="editing !== rec.date && rec.events && rec.events.length > 0" style="font-size:20px;margin-top:4px;letter-spacing:2px">
          <span v-for="ev in rec.events" :key="ev.id">{{ ev.event_type === 'excretion' ? '🚽' : '⚖️' }}</span>
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
        <p v-if="settings.length === 0 && !editing" style="color:#999;font-size:14px">設定がありません</p>
        <table v-if="settings.length > 0">
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
