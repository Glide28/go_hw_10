const API_BASE = 'http://localhost:8080';

function login(email, password) {
  const response = UrlFetchApp.fetch(API_BASE + '/api/auth/login', {
    method: 'post',
    contentType: 'application/json',
    payload: JSON.stringify({ email, password }),
  });
  const token = JSON.parse(response.getContentText()).token;
  PropertiesService.getUserProperties().setProperty('CASHCRAFT_TOKEN', token);
}

function addTransactionFromActiveRow() {
  const sheet = SpreadsheetApp.getActive().getSheetByName('Transactions');
  const row = sheet.getActiveRange().getRow();
  const [amount, category, description, date] = sheet.getRange(row, 1, 1, 4).getValues()[0];
  apiFetch('/api/transactions', 'post', {
    amount: Number(amount),
    category: String(category),
    description: String(description),
    date: Utilities.formatDate(new Date(date), 'GMT', 'yyyy-MM-dd'),
  });
}

function loadMonthlyReport() {
  const sheet = SpreadsheetApp.getActive().getSheetByName('Report');
  const from = Utilities.formatDate(new Date(sheet.getRange('B1').getValue()), 'GMT', 'yyyy-MM-dd');
  const to = Utilities.formatDate(new Date(sheet.getRange('B2').getValue()), 'GMT', 'yyyy-MM-dd');
  const report = apiFetch('/api/reports/summary?from=' + from + '&to=' + to, 'get');
  sheet.getRange(4, 1, Math.max(sheet.getMaxRows() - 3, 1), 2).clearContent();
  if (report.length) {
    sheet.getRange(4, 1, report.length, 2).setValues(report.map(x => [x.category, x.total]));
  }
}

function apiFetch(path, method, payload) {
  const token = PropertiesService.getUserProperties().getProperty('CASHCRAFT_TOKEN');
  const options = {
    method,
    headers: { Authorization: 'Bearer ' + token },
    muteHttpExceptions: false,
  };
  if (payload !== undefined) {
    options.contentType = 'application/json';
    options.payload = JSON.stringify(payload);
  }
  const response = UrlFetchApp.fetch(API_BASE + path, options);
  return JSON.parse(response.getContentText());
}
