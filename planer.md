// ============================================================
// ПЛАНЕР v4 — Google Apps Script
// Расширения → Apps Script → вставить → Сохранить → setupPlanner
// ============================================================

var DAYS = 60;
var CAL_META = 4; // Задача | Тема | Начало | Длит.

var DURATION_LIST = [
  '5 мин','10 мин','15 мин','20 мин','25 мин','30 мин',
  '45 мин','1 ч','1.5 ч','2 ч','2.5 ч','3 ч','4 ч','5 ч','Весь день'
];

var START_TIMES = [
  '06:00','06:30','07:00','07:30','08:00','08:30',
  '09:00','09:30','10:00','10:30','11:00','11:30',
  '12:00','12:30','13:00','13:30','14:00','14:30',
  '15:00','15:30','16:00','16:30','17:00','17:30',
  '18:00','18:30','19:00','19:30','20:00','20:30',
  '21:00','21:30','22:00','22:30','23:00','23:30'
];

var FORMATS = [
  'Один раз',
  'Ежедневно',
  'Пн–Пт (будни)',
  'Сб–Вс (выходные)',
  'Пн / Ср / Пт',
  'Вт / Чт / Сб',
  'Через день',
  'Раз в неделю',
  'Раз в 2 недели',
  'Раз в месяц',
  'Закрепление: 0→1→3→7→14→30→90',
  'Закрепление Ebbinghaus: 0→1→3→7→14→30→60→90→180→365',
  'Закрепление интенсив: 0→1→2→4→7→14→21→30→60→90',
  'Закрепление Anki: 0→1→4→10→25→60→150'
];

// Интервалы spaced repetition (дни от startDate)
var SR = {};
SR['Закрепление: 0→1→3→7→14→30→90']                    = [0,1,3,7,14,30,90];
SR['Закрепление Ebbinghaus: 0→1→3→7→14→30→60→90→180→365'] = [0,1,3,7,14,30,60,90,180,365];
SR['Закрепление интенсив: 0→1→2→4→7→14→21→30→60→90']   = [0,1,2,4,7,14,21,30,60,90];
SR['Закрепление Anki: 0→1→4→10→25→60→150']              = [0,1,4,10,25,60,150];

// Возвращает "yyyy-MM-dd" в таймзоне скрипта
function fmtDate(d) {
  return Utilities.formatDate(d, Session.getScriptTimeZone(), 'yyyy-MM-dd');
}

// Строит дату (без времени, только год/месяц/день) из строки "yyyy-MM-dd"
function parseDate(s) {
  var p = s.split('-').map(Number);
  return new Date(p[0], p[1]-1, p[2]);
}

// "Сегодня" без времени, правильно по таймзоне
function getToday() {
  return parseDate(fmtDate(new Date()));
}

// ─── SETUP ───────────────────────────────────────────────────
function setupPlanner() {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  ['📝 Задачи','📅 Календарь','⚙️ Темы'].forEach(function(n) {
    var s = ss.getSheetByName(n); if (s) ss.deleteSheet(s);
  });
  createTopicsSheet(ss);
  createTasksSheet(ss);
  createCalendarSheet(ss);
  ss.setActiveSheet(ss.getSheetByName('📝 Задачи'));
  createMenu();
  SpreadsheetApp.getUi().alert(
    '✅ Планер готов!\n\n' +
    '1. В "📝 Задачи" добавь задачи\n' +
    '2. Поставь ✓ в колонке "Вкл"\n' +
    '3. Меню "🗓 Планер" → "Обновить календарь"\n' +
    '4. В "📅 Календарь" ставь ✓ когда выполнил\n' +
    '5. Пропустил? → "Перенести невыполненные" — все повторения сдвинутся\n\n' +
    'Отключил задачу? Нажми "Обновить календарь" — она исчезнет.'
  );
}

function onOpen() { createMenu(); }

function createMenu() {
  SpreadsheetApp.getUi().createMenu('🗓 Планер')
    .addItem('🔄 Обновить календарь', 'generateCalendar')
    .addItem('⏩ Перенести невыполненные', 'rescheduleUnchecked')
    .addSeparator()
    .addItem('🔧 Пересоздать планер', 'setupPlanner')
    .addToUi();
}

// ─── ЛИСТ: ТЕМЫ ──────────────────────────────────────────────
function createTopicsSheet(ss) {
  var sh = ss.insertSheet('⚙️ Темы');
  sh.setTabColor('#9C27B0');
  sh.setColumnWidth(1, 200); sh.setColumnWidth(2, 320);

  sh.setRowHeight(1, 30);
  sh.getRange(1,1).setValue('Тема').setFontWeight('bold').setBackground('#7B1FA2').setFontColor('#fff').setHorizontalAlignment('center');
  sh.getRange(1,2).setValue('Описание').setFontWeight('bold').setBackground('#7B1FA2').setFontColor('#fff');

  var topics = [
    ['Go / Golang',   'Горутины, каналы, интерфейсы, concurrency'],
    ['Python',        'Скрипты, автоматизация, ML'],
    ['JavaScript',    'Frontend, Node.js, TypeScript'],
    ['Английский',    'Лексика, грамматика, speaking'],
    ['Математика',    'Алгебра, статистика, дискретная математика'],
    ['Работа',        'Рабочие задачи и встречи'],
    ['Спорт',         'Тренировки, зарядка, пробежка'],
    ['Чтение',        'Книги, статьи, документация'],
    ['Личное',        'Личные дела и планы'],
    ['Финансы',       'Бюджет, инвестиции, расходы'],
  ];
  topics.forEach(function(t, i) {
    sh.setRowHeight(2+i, 26);
    sh.getRange(2+i,1).setValue(t[0]).setFontSize(11);
    sh.getRange(2+i,2).setValue(t[1]).setFontSize(10).setFontColor('#5F6368');
    sh.getRange(2+i,1,1,2).setBackground(i%2===0?'#fff':'#F8F9FA');
  });
  if (sh.getMaxColumns() > 2) sh.hideColumns(3, sh.getMaxColumns()-2);
  sh.hideRows(topics.length+3, sh.getMaxRows()-topics.length-2);
}

// ─── ЛИСТ: ЗАДАЧИ ────────────────────────────────────────────
// Колонки: 1=Вкл 2=Задача 3=Тема 4=Формат 5=Начало 6=Длит. 7=С какого дня 8=Заметка
function createTasksSheet(ss) {
  var sh = ss.insertSheet('📝 Задачи');
  sh.setTabColor('#4F86F7');
  [40,300,160,220,80,100,115,220].forEach(function(w,i){ sh.setColumnWidth(i+1,w); });

  sh.setRowHeight(1,44);
  sh.getRange(1,1,1,8).merge().setValue('📝 МОИ ЗАДАЧИ')
    .setBackground('#1A1A2E').setFontColor('#fff').setFontSize(13).setFontWeight('bold')
    .setHorizontalAlignment('center').setVerticalAlignment('middle');

  sh.setRowHeight(2,30);
  ['Вкл','Задача','Тема','Формат повторения','Начало','Длит.','С какого дня','Заметка'].forEach(function(h,i){
    sh.getRange(2,i+1).setValue(h).setBackground('#16213E').setFontColor('#A8DADC')
      .setFontWeight('bold').setFontSize(10).setHorizontalAlignment('center').setVerticalAlignment('middle');
  });
  sh.setFrozenRows(2);

  var topicSh = ss.getSheetByName('⚙️ Темы');
  var topicCount = topicSh.getLastRow()-1;
  var topics = [];
  topicSh.getRange(2,1,topicCount,1).getValues().forEach(function(r){ if(r[0]) topics.push(r[0]); });

  var todayStr = fmtDate(getToday());
  for (var i = 0; i < 30; i++) {
    var row = i+3;
    sh.setRowHeight(row, 30);
    sh.getRange(row,1,1,8).setBackground(i%2===0?'#fff':'#F8F9FA').setVerticalAlignment('middle');
    sh.getRange(row,1).insertCheckboxes().setValue(false);
    sh.getRange(row,5).setHorizontalAlignment('center').setFontSize(10);
    sh.getRange(row,6).setHorizontalAlignment('center').setFontSize(10);
    sh.getRange(row,7).setValue(todayStr).setNumberFormat('dd.MM.yyyy')
      .setHorizontalAlignment('center').setFontSize(10);
    sh.getRange(row,8).setFontSize(10).setFontColor('#5F6368');
    if (topics.length > 0) addDropdown(sh, row, 3, topics);
    addDropdown(sh, row, 4, FORMATS);
    addDropdown(sh, row, 5, START_TIMES);
    addDropdown(sh, row, 6, DURATION_LIST);
  }

  sh.getRange(2,1,31,8).setBorder(true,true,true,true,true,true,'#E8EAED',SpreadsheetApp.BorderStyle.SOLID);
  sh.setRowHeight(34,36);
  sh.getRange(34,1,1,8).merge()
    .setValue('💡 Заполни задачу → ✓ "Вкл" → меню "🗓 Планер → Обновить календарь". При отключении задачи снова нажми "Обновить".')
    .setBackground('#FFF9C4').setFontColor('#5D4037').setFontSize(10).setWrap(true).setVerticalAlignment('middle');
  if (sh.getMaxColumns() > 8) sh.hideColumns(9, sh.getMaxColumns()-8);
  sh.hideRows(35, sh.getMaxRows()-34);
}

// ─── ЛИСТ: КАЛЕНДАРЬ ─────────────────────────────────────────
// Row 1: [Title A1:D1 merged] + дата (число) в E1, F1...
// Row 2: [Задача Тема Начало ⏱] + день недели в E2, F2...
// Freeze: rows=2, cols=4 (вызываем ПОСЛЕДНИМИ чтобы не конфликтовать с merge)
function createCalendarSheet(ss) {
  var sh = ss.insertSheet('📅 Календарь');
  sh.setTabColor('#34A853');

  var totalCols = CAL_META + DAYS;
  if (sh.getMaxColumns() < totalCols) {
    sh.insertColumnsAfter(sh.getMaxColumns(), totalCols - sh.getMaxColumns());
  }

  sh.setColumnWidth(1, 260); sh.setColumnWidth(2, 120);
  sh.setColumnWidth(3, 65);  sh.setColumnWidth(4, 75);
  for (var c = CAL_META+1; c <= totalCols; c++) sh.setColumnWidth(c, 38);

  sh.setRowHeight(1, 42);
  // Merge только A1:D1 (внутри будущих frozen cols — конфликта нет)
  sh.getRange(1,1,1,CAL_META).merge()
    .setValue('📅 КАЛЕНДАРЬ ЗАДАЧ')
    .setBackground('#1A1A2E').setFontColor('#fff').setFontSize(13).setFontWeight('bold')
    .setHorizontalAlignment('left').setVerticalAlignment('middle');

  var today = getToday();
  var dowRu = ['Вс','Пн','Вт','Ср','Чт','Пт','Сб'];

  for (var d = 0; d < DAYS; d++) {
    var date = parseDate(fmtDate(new Date(today.getFullYear(), today.getMonth(), today.getDate()+d)));
    var col = CAL_META+1+d;
    var dow = date.getDay();
    var isWeekend = dow===0||dow===6;
    var isToday   = d===0;

    // Row 1: сохраняем DATE-значение (нужно для CF E$1 < TODAY())
    sh.getRange(1,col).setValue(date).setNumberFormat('d')
      .setBackground(isToday?'#4F86F7':isWeekend?'#2C1A2E':'#1A1A2E')
      .setFontColor(isToday?'#fff':isWeekend?'#E8A0BF':'#A8DADC')
      .setFontSize(11).setFontWeight('bold').setHorizontalAlignment('center').setVerticalAlignment('middle');

    // Row 2: день недели
    sh.getRange(2,col).setValue(dowRu[dow])
      .setBackground(isWeekend?'#3A1A2E':'#16213E')
      .setFontColor(isWeekend?'#E8A0BF':'#5F6368')
      .setFontSize(7).setHorizontalAlignment('center').setVerticalAlignment('middle');
  }

  sh.setRowHeight(2, 18);

  // Заголовки мета-колонок (row 2, cols 1-4)
  ['Задача','Тема','Начало','⏱'].forEach(function(h,i){
    sh.getRange(2,i+1).setValue(h).setBackground('#16213E').setFontColor('#A8DADC')
      .setFontWeight('bold').setFontSize(9).setHorizontalAlignment('center').setVerticalAlignment('middle');
  });

  // Freeze ПОСЛЕДНИМИ (после всех merge и setValue)
  sh.setFrozenRows(2);
  sh.setFrozenColumns(CAL_META);

  if (sh.getMaxColumns() > totalCols) sh.hideColumns(totalCols+1, sh.getMaxColumns()-totalCols);
}

// ─── ГЕНЕРАЦИЯ КАЛЕНДАРЯ ─────────────────────────────────────
function generateCalendar() {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var taskSh = ss.getSheetByName('📝 Задачи');
  var calSh  = ss.getSheetByName('📅 Календарь');
  if (!taskSh || !calSh) {
    SpreadsheetApp.getUi().alert('Листы не найдены. Запусти "Пересоздать планер".');
    return;
  }

  var today = getToday();

  // Читаем задачи
  var lastTaskRow = taskSh.getLastRow();
  if (lastTaskRow < 3) { SpreadsheetApp.getUi().alert('Нет задач.'); return; }

  var rawTasks = taskSh.getRange(3,1,lastTaskRow-2,8).getValues();
  // Только активные (Вкл=TRUE) задачи
  var tasks = rawTasks.filter(function(r){ return r[0]===true && String(r[1]).trim()!==''; });

  if (tasks.length === 0) {
    // Очистить всё — все задачи отключены
    var calLastRow = calSh.getLastRow();
    if (calLastRow > 2) calSh.getRange(3,1,calLastRow-2,CAL_META+DAYS).clearContent().clearDataValidations().setBackground(null);
    SpreadsheetApp.getUi().alert('Нет активных задач. Поставь ✓ в колонке "Вкл".');
    return;
  }

  // Сортируем по времени начала (без времени — в конец)
  tasks.sort(function(a,b){
    var ta = timeStr(a[4]); var tb = timeStr(b[4]);
    return ta < tb ? -1 : ta > tb ? 1 : 0;
  });

  var totalCols = CAL_META + DAYS;

  // Сохраняем существующие состояния чекбоксов по [taskName][dateStr]
  var saved = {};
  var calLastRow = calSh.getLastRow();
  var oldDateHeaders = calSh.getRange(1,CAL_META+1,1,DAYS).getValues()[0];

  if (calLastRow >= 3) {
    calSh.getRange(3,1,calLastRow-2,totalCols).getValues().forEach(function(row){
      var name = String(row[0]).trim();
      if (!name) return;
      saved[name] = {};
      for (var d = 0; d < DAYS; d++) {
        var dh = oldDateHeaders[d];
        if (dh) saved[name][fmtDate(new Date(dh))] = row[CAL_META+d];
      }
    });
  }

  // Очищаем строки данных
  if (calLastRow > 2) {
    calSh.getRange(3,1,calLastRow-2,totalCols)
      .clearContent().clearDataValidations()
      .setBackground(null).setFontColor(null);
  }

  // Добавляем строки если нужно
  if (calSh.getMaxRows() < tasks.length+3) {
    calSh.insertRowsAfter(calSh.getMaxRows(), tasks.length+3 - calSh.getMaxRows());
  }

  // Обновляем заголовки дат (row 1 + row 2) — сегодня могло сдвинуться
  var dowRu = ['Вс','Пн','Вт','Ср','Чт','Пт','Сб'];
  for (var d = 0; d < DAYS; d++) {
    var date = parseDate(fmtDate(new Date(today.getFullYear(), today.getMonth(), today.getDate()+d)));
    var col = CAL_META+1+d;
    var dow = date.getDay();
    var isWeekend = dow===0||dow===6;
    var isToday   = d===0;
    calSh.getRange(1,col).setValue(date).setNumberFormat('d')
      .setBackground(isToday?'#4F86F7':isWeekend?'#2C1A2E':'#1A1A2E')
      .setFontColor(isToday?'#fff':isWeekend?'#E8A0BF':'#A8DADC')
      .setFontSize(11).setFontWeight('bold').setHorizontalAlignment('center').setVerticalAlignment('middle');
    calSh.getRange(2,col).setValue(dowRu[dow])
      .setBackground(isWeekend?'#3A1A2E':'#16213E')
      .setFontColor(isWeekend?'#E8A0BF':'#5F6368')
      .setFontSize(7).setHorizontalAlignment('center').setVerticalAlignment('middle');
  }

  // Записываем строки задач
  var toCheck = [];  // { notation, row, col, prevVal }

  tasks.forEach(function(task, idx){
    var row = idx+3;
    var name     = String(task[1]).trim();
    var topic    = String(task[2]);
    var format   = String(task[3]) || 'Один раз';
    var startT   = timeStr(task[4]);
    var dur      = task[5] ? String(task[5]) : '';
    var startD   = task[6] ? parseDate(fmtDate(new Date(task[6]))) : getToday();

    calSh.setRowHeight(row, 30);
    calSh.getRange(row,1,1,totalCols).setBackground(idx%2===0?'#fff':'#F8F9FA').setVerticalAlignment('middle');
    calSh.getRange(row,1).setValue(name).setFontSize(11);
    calSh.getRange(row,2).setValue(topic).setFontSize(9).setFontColor('#5F6368').setHorizontalAlignment('center');
    calSh.getRange(row,3).setValue(startT==='99:99'?'':startT).setFontSize(9).setFontColor('#5F6368').setHorizontalAlignment('center');
    calSh.getRange(row,4).setValue(dur).setFontSize(9).setFontColor('#5F6368').setHorizontalAlignment('center');

    var prevSaved = saved[name] || {};

    for (var d = 0; d < DAYS; d++) {
      var date = parseDate(fmtDate(new Date(today.getFullYear(), today.getMonth(), today.getDate()+d)));
      if (isScheduled(format, startD, date)) {
        var col = CAL_META+1+d;
        var dStr = fmtDate(date);
        toCheck.push({ n: calSh.getRange(row,col).getA1Notation(), row:row, col:col, prev: prevSaved[dStr] });
      }
    }
  });

  // Вставляем чекбоксы пачками
  for (var b = 0; b < toCheck.length; b += 500) {
    calSh.getRangeList(toCheck.slice(b,b+500).map(function(x){return x.n;})).insertCheckboxes();
  }
  // Восстанавливаем выполненные (TRUE)
  toCheck.forEach(function(x){ if(x.prev===true) calSh.getRange(x.row,x.col).setValue(true); });

  // Скрываем лишние строки
  var usedRows = tasks.length+2;
  if (calSh.getMaxRows() > usedRows) calSh.hideRows(usedRows+1, calSh.getMaxRows()-usedRows);
  calSh.showRows(3, tasks.length);

  // Условное форматирование
  var dataRange = calSh.getRange(3, CAL_META+1, tasks.length, DAYS);
  var cfDone = SpreadsheetApp.newConditionalFormatRule()
    .whenFormulaSatisfied('=E3=TRUE')
    .setBackground('#E6F4EA').setFontColor('#137333')
    .setRanges([dataRange]).build();
  var cfMissed = SpreadsheetApp.newConditionalFormatRule()
    .whenFormulaSatisfied('=AND(E3=FALSE,E$1<TODAY(),NOT(ISBLANK(E3)))')
    .setBackground('#FCE8E6').setFontColor('#C5221F')
    .setRanges([dataRange]).build();
  calSh.setConditionalFormatRules([cfDone, cfMissed]);

  SpreadsheetApp.getUi().alert('✅ Календарь обновлён! ' + tasks.length + ' задач · ' + DAYS + ' дней\n\n🟩 Выполнено  🟥 Пропущено');
}

// ─── ЛОГИКА РАСПИСАНИЯ ───────────────────────────────────────
function isScheduled(format, startDate, checkDate) {
  var sStr = fmtDate(startDate);
  var cStr = fmtDate(checkDate);
  var sp = sStr.split('-').map(Number);
  var cp = cStr.split('-').map(Number);
  // Используем UTC чтобы избежать DST-скачков при вычислении diff
  var diff = Math.round((Date.UTC(cp[0],cp[1]-1,cp[2]) - Date.UTC(sp[0],sp[1]-1,sp[2])) / 86400000);
  if (diff < 0) return false;

  var dow = checkDate.getDay(); // 0=Вс, 1=Пн, ..., 6=Сб

  if (format === 'Один раз')          return diff === 0;
  if (format === 'Ежедневно')         return true;
  if (format === 'Пн–Пт (будни)')    return dow >= 1 && dow <= 5;
  if (format === 'Сб–Вс (выходные)') return dow === 0 || dow === 6;
  if (format === 'Пн / Ср / Пт')     return dow === 1 || dow === 3 || dow === 5;
  if (format === 'Вт / Чт / Сб')     return dow === 2 || dow === 4 || dow === 6;
  if (format === 'Через день')        return diff % 2 === 0;
  if (format === 'Раз в неделю')      return diff % 7 === 0;
  if (format === 'Раз в 2 недели')    return diff % 14 === 0;
  if (format === 'Раз в месяц')       return diff % 30 === 0;

  var intervals = SR[format];
  if (intervals) return intervals.indexOf(diff) >= 0;

  return diff === 0;
}

// ─── ПЕРЕНОС НЕВЫПОЛНЕННЫХ ───────────────────────────────────
// Находит первую пропущенную дату для каждой задачи,
// сдвигает startDate на нужное кол-во дней → ВСЕ повторения съезжают вперёд
function rescheduleUnchecked() {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var calSh  = ss.getSheetByName('📅 Календарь');
  var taskSh = ss.getSheetByName('📝 Задачи');
  if (!calSh || !taskSh) return;

  var today    = getToday();
  var todayStr = fmtDate(today);

  var calLastRow = calSh.getLastRow();
  if (calLastRow < 3) { SpreadsheetApp.getUi().alert('Календарь пуст.'); return; }

  // Читаем даты заголовков (row 1)
  var dateHeaders = calSh.getRange(1,CAL_META+1,1,DAYS).getValues()[0];

  // Индекс сегодняшней колонки
  var todayOffset = -1;
  for (var d = 0; d < DAYS; d++) {
    if (dateHeaders[d] && fmtDate(new Date(dateHeaders[d])) === todayStr) {
      todayOffset = d; break;
    }
  }
  if (todayOffset < 0) {
    SpreadsheetApp.getUi().alert('Не найдена сегодняшняя дата. Сначала нажми "Обновить календарь".');
    return;
  }

  var taskLastRow = taskSh.getLastRow();
  var taskData    = taskSh.getRange(3,1,taskLastRow-2,8).getValues();
  var calData     = calSh.getRange(3,1,calLastRow-2,CAL_META+DAYS).getValues();

  var shifted = 0;

  calData.forEach(function(calRow){
    var taskName = String(calRow[0]).trim();
    if (!taskName) return;

    // Ищем первую невыполненную (FALSE) ячейку в прошедших днях
    var firstMissedOffset = -1;
    for (var d = 0; d < todayOffset; d++) {
      // FALSE = чекбокс есть, но не отмечен
      if (calRow[CAL_META+d] === false) { firstMissedOffset = d; break; }
    }
    if (firstMissedOffset < 0) return;

    // Сдвиг в днях: сегодня - день пропуска
    var missedDate = new Date(dateHeaders[firstMissedOffset]);
    var shiftDays  = Math.round((today.getTime() - missedDate.getTime()) / 86400000);
    if (shiftDays <= 0) return;

    // Находим задачу в Tasks sheet по имени и обновляем startDate
    for (var t = 0; t < taskData.length; t++) {
      if (String(taskData[t][1]).trim() === taskName && taskData[t][0] === true) {
        var oldStart = taskData[t][6];
        if (!oldStart) return;
        var oldStartDate = parseDate(fmtDate(new Date(oldStart)));
        var newStartMs   = oldStartDate.getTime() + shiftDays * 86400000;
        var newStartDate = new Date(newStartMs);
        taskSh.getRange(t+3,7).setValue(newStartDate).setNumberFormat('dd.MM.yyyy');
        shifted++;
        break;
      }
    }
  });

  if (shifted > 0) {
    generateCalendar();
  } else {
    SpreadsheetApp.getUi().alert('✅ Нет пропущенных задач! Всё выполнено.');
  }
}

// ─── HELPERS ─────────────────────────────────────────────────
function timeStr(val) {
  if (!val) return '99:99';
  if (val instanceof Date) return Utilities.formatDate(val, Session.getScriptTimeZone(), 'HH:mm');
  return String(val);
}

function addDropdown(sh, row, col, items) {
  if (!items || !items.length) return;
  sh.getRange(row,col).setDataValidation(
    SpreadsheetApp.newDataValidation().requireValueInList(items,true).setAllowInvalid(false).build()
  );
}