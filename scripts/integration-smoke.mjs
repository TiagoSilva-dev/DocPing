#!/usr/bin/env node
/**
 * Real local-infrastructure smoke test. Requires healthy Docker Compose services.
 * Uses only loopback API/MinIO/Mailpit endpoints and synthetic example.invalid data.
 * Never prints passwords, session cookies, public links, document contents or DSNs.
 *
 * node scripts/integration-smoke.mjs [--mysql] [--eicar]
 * --mysql runs the opt-in Go MySQL persistence test, passing its DSN only via env.
 * --eicar also tests the harmless antivirus test signature embedded in a PDF.
 */
import assert from 'node:assert/strict';
import { randomBytes, randomUUID } from 'node:crypto';
import { readFile, mkdir, writeFile } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('..', import.meta.url));
const dotEnv = {};
try {
  for (const raw of (await readFile(path.join(root, '.env'), 'utf8')).split(/\r?\n/)) {
    const line = raw.trim().replace(/^export\s+/, '');
    const match = /^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/.exec(line);
    if (!match) continue;
    let value = match[2].trim();
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) value = value.slice(1, -1);
    else value = value.replace(/\s+#.*$/, '');
    dotEnv[match[1]] = value;
  }
} catch (error) { if (error.code !== 'ENOENT') throw new Error('Unable to read local integration configuration.'); }
const config = (name, fallback = '') => process.env[name] ?? dotEnv[name] ?? fallback;
function loopback(value, label) {
  let parsed;
  try { parsed = new URL(value); } catch { throw new Error(`${label} must be a valid local URL.`); }
  assert(['localhost', '127.0.0.1', '[::1]'].includes(parsed.hostname) && ['http:', 'https:'].includes(parsed.protocol) && !parsed.username && !parsed.password, `${label} must use loopback without URL credentials.`);
  return parsed.origin;
}
const apiBase = loopback(config('INTEGRATION_API_URL', `http://127.0.0.1:${config('API_HOST_PORT', '8080')}`), 'API');
const minioBase = loopback(config('INTEGRATION_MINIO_URL', 'http://127.0.0.1:9000'), 'MinIO');
const mailpitBase = loopback(config('INTEGRATION_MAILPIT_URL', 'http://127.0.0.1:8025'), 'Mailpit');
const webOrigin = loopback(config('WEB_ORIGIN', 'http://localhost:5173'), 'WEB_ORIGIN');
const runID = randomUUID();
const checks = [];
const started = Date.now();
let stage = 'preflight';
async function check(name, fn) {
  stage = name;
  const begin = Date.now();
  await fn();
  checks.push({ name, status: 'passed', elapsedMs: Date.now() - begin });
  console.log(`PASS ${name}`);
}
class API {
  cookies = new Map();
  csrf = '';
  async request(method, route, { json, form, expected = 200, binary = false } = {}) {
    const headers = { Origin: webOrigin };
    if (this.cookies.size) headers.Cookie = [...this.cookies].map(([name, value]) => `${name}=${value}`).join('; ');
    if (this.csrf && !['GET', 'HEAD'].includes(method)) headers['X-CSRF-Token'] = this.csrf;
    let body;
    if (json !== undefined) { headers['Content-Type'] = 'application/json'; body = JSON.stringify(json); }
    if (form) body = form;
    const response = await fetch(`${apiBase}/api${route}`, { method, headers, body, signal: AbortSignal.timeout(90_000), redirect: 'error' });
    const setCookies = response.headers.getSetCookie?.() ?? [response.headers.get('set-cookie')].filter(Boolean);
    for (const cookie of setCookies) {
      const pair = cookie.split(';', 1)[0];
      const split = pair.indexOf('=');
      if (split > 0) this.cookies.set(pair.slice(0, split), pair.slice(split + 1));
    }
    let data;
    if (binary) data = Buffer.from(await response.arrayBuffer());
    else {
      try { data = await response.json(); } catch { throw new Error(`${stage}: API response was not JSON (HTTP ${response.status}).`); }
    }
    if (response.status !== expected) {
      const code = !binary && data?.error?.code ? ` (${data.error.code})` : '';
      throw new Error(`${stage}: expected HTTP ${expected}, received ${response.status}${code}.`);
    }
    if (!binary && data?.csrfToken) this.csrf = data.csrfToken;
    return { data, response };
  }
}
const office = new API();
const anonymous = new API();
const stranger = new API();
const portalBrowser = new API();
const officeEmail = `office-${runID}@example.invalid`;
const clientEmail = `client-${runID}@example.invalid`;
const password = `${randomBytes(24).toString('base64url')}aA9!`;
const organizationName = `DocPing integration ${runID.slice(0, 8)}`;

function syntheticPDF(revision = 'original', attachment = null) {
  const content = `BT /F1 14 Tf 48 740 Td (DOCPING - SYNTHETIC INTEGRATION SAMPLE) Tj 0 -24 Td /F1 10 Tf (No real company or personal data. Revision ${revision}.) Tj ET`;
  const objects = ['<< /Type /Catalog /Pages 2 0 R >>', '<< /Type /Pages /Kids [3 0 R] /Count 1 >>', '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>', '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>', `<< /Length ${Buffer.byteLength(content)} >>\nstream\n${content}\nendstream`];
  if (attachment) {
    objects[0] = '<< /Type /Catalog /Pages 2 0 R /Names << /EmbeddedFiles << /Names [(eicar.com) 6 0 R] >> >> /AF [6 0 R] >>';
    objects.push('<< /Type /Filespec /F (eicar.com) /UF (eicar.com) /EF << /F 7 0 R >> /AFRelationship /Data >>', `<< /Type /EmbeddedFile /Length ${attachment.length} >>\nstream\n${attachment.toString('ascii')}\nendstream`);
  }
  let out = '%PDF-1.4\n';
  const offsets = [];
  objects.forEach((object, index) => { offsets.push(Buffer.byteLength(out)); out += `${index + 1} 0 obj\n${object}\nendobj\n`; });
  const xref = Buffer.byteLength(out);
  out += `xref\n0 ${objects.length + 1}\n0000000000 65535 f \n` + offsets.map(offset => `${String(offset).padStart(10, '0')} 00000 n \n`).join('');
  out += `trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`;
  return Buffer.from(out);
}
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGNQzIn5DwADhAHpnYSMogAAAABJRU5ErkJggg==', 'base64');
async function upload(api, token, itemID, filename, mime, bytes, expected = 200) {
  const form = new FormData();
  form.set('file', new Blob([bytes], { type: mime }), filename);
  return (await api.request('POST', `/portal/${token}/items/${itemID}/upload`, { form, expected })).data;
}
async function mailFor(recipient, subject) {
  // The Mailpit list/search API is read-only. No message release/relay is used.
  const search = new URL('/api/v1/search', mailpitBase);
  search.searchParams.set('query', `to:${recipient}`);
  for (let attempt = 0; attempt < 12; attempt++) {
    const response = await fetch(search, { signal: AbortSignal.timeout(5_000) });
    if (!response.ok) throw new Error(`Mailpit search failed (HTTP ${response.status}).`);
    const payload = await response.json();
    const messages = payload.messages ?? payload.Messages ?? [];
    const selected = messages.find(message => (message.Subject ?? message.subject) === subject);
    if (selected) {
      const id = selected.ID ?? selected.Id ?? selected.id;
      const detail = await fetch(`${mailpitBase}/api/v1/message/${encodeURIComponent(id)}`, { signal: AbortSignal.timeout(5_000) });
      assert.equal(detail.status, 200, 'Mailpit message detail unavailable.');
      return await detail.json();
    }
    await new Promise(resolve => setTimeout(resolve, 300));
  }
  throw new Error('Mailpit did not contain the SMTP-accepted synthetic message.');
}
async function register(api, email, name) {
  return (await api.request('POST', '/auth/register', { json: { organizationName: name, name: 'Integration tester', email, password } })).data;
}
let session, client, template, request, token, originalFile, pngFile, sensitive, sensitiveToken;
try {
  await check('local services respond', async () => {
    const api = await office.request('GET', '/health');
    assert.equal(api.data.status, 'ok');
    const minio = await fetch(`${minioBase}/minio/health/live`, { signal: AbortSignal.timeout(5_000) });
    assert.equal(minio.status, 200, 'MinIO is not healthy.');
    const mailpit = await fetch(`${mailpitBase}/api/v1/messages`, { signal: AbortSignal.timeout(5_000) });
    assert.equal(mailpit.status, 200, 'Mailpit API is not healthy.');
  });
  await check('signup, session cookie and empty real organization', async () => {
    session = await register(office, officeEmail, organizationName);
    assert.equal(session.organization.demo, false);
    assert(office.cookies.has('docping_session') && office.csrf, 'Session cookie/CSRF token missing.');
    const current = (await office.request('GET', '/session')).data;
    assert.equal(current.organization.id, session.organization.id);
    assert.deepEqual((await office.request('GET', '/clients')).data, []);
    assert.deepEqual((await office.request('GET', '/requests')).data, []);
  });
  await check('client and reusable template persisted', async () => {
    client = (await office.request('POST', '/clients', { json: { name: `Synthetic company ${runID.slice(0, 8)}`, contactName: 'Synthetic contact', email: clientEmail, cnpj: '', phone: '' } })).data;
    template = (await office.request('POST', '/templates', { json: { name: 'Synthetic integration documents', description: 'No real client documents.', items: [{ name: 'Synthetic statement', instruction: 'Synthetic PDF and PNG only.', required: true, maxFiles: 3 }] } })).data;
    assert.equal((await office.request('GET', `/clients/${client.id}`)).data.id, client.id);
    assert.equal((await office.request('GET', `/templates/${template.id}`)).data.items[0].name, 'Synthetic statement');
  });
  await check('CSRF is enforced on authenticated mutations', async () => {
    const saved = office.csrf; office.csrf = '';
    try { await office.request('POST', '/clients', { json: { name: 'Should never be created' }, expected: 403 }); }
    finally { office.csrf = saved; }
  });
  await check('request, snapshot and public link persisted', async () => {
    const now = new Date();
    const competence = `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, '0')}`;
    const dueDate = new Date(Date.now() + 14 * 86400_000).toISOString().slice(0, 10);
    const created = (await office.request('POST', '/requests', { json: { clientIds: [client.id], templateId: template.id, title: 'Synthetic integration collection', competence, dueDate } })).data;
    assert.equal(created.length, 1); request = created[0];
    assert.equal(request.status, 'pending');
    token = (await office.request('POST', `/requests/${request.id}/share`)).data.token;
    assert.equal(token.length, 64);
    const portal = (await anonymous.request('GET', `/portal/${token}`)).data;
    assert.equal(portal.request.id, request.id);
    assert(portal.maxUploadBytes > 0);
  });
  await check('PDF passes actual ClamAV and private MinIO round trip', async () => {
    request = await upload(anonymous, token, request.items[0].id, 'synthetic-statement.pdf', 'application/pdf', syntheticPDF());
    originalFile = request.items[0].files[0];
    assert.equal(originalFile.scanStatus, 'clean', 'Scanner bypass is forbidden in this integration smoke.');
    assert(!originalFile.sample && !originalFile.objectKey, 'Uploaded file must be real private storage, without leaked object key.');
    const downloaded = (await office.request('GET', `/files/${originalFile.id}`, { binary: true })).data;
    assert(downloaded.equals(syntheticPDF()), 'Private object bytes changed.');
  });
  await check('PNG passes actual ClamAV and independent upload', async () => {
    request = await upload(anonymous, token, request.items[0].id, 'synthetic-pixel.png', 'image/png', png);
    assert.equal(request.items[0].files.length, 2);
    pngFile = request.items[0].files.find(file => file.mimeType === 'image/png');
    assert.equal(pngFile.scanStatus, 'clean');
    const downloaded = (await anonymous.request('GET', `/files/${pngFile.id}?token=${encodeURIComponent(token)}`, { binary: true })).data;
    assert(downloaded.equals(png), 'PNG object bytes changed.');
  });
  await check('unauthenticated file and unsigned MinIO object denied', async () => {
    await anonymous.request('GET', `/files/${originalFile.id}`, { expected: 401 });
    const bucket = config('S3_BUCKET', 'docping');
    const object = [bucket, session.organization.id, request.id, originalFile.id].map(encodeURIComponent).join('/');
    const response = await fetch(`${minioBase}/${object}`, { signal: AbortSignal.timeout(5_000), redirect: 'error' });
    assert.equal(response.status, 403, 'Private object unexpectedly allows unsigned access.');
  });
  await check('unrelated organization cannot query, mutate or download', async () => {
    await register(stranger, `other-${runID}@example.invalid`, `Other integration ${runID.slice(0, 8)}`);
    await stranger.request('GET', `/requests/${request.id}`, { expected: 404 });
    await stranger.request('GET', `/clients/${client.id}`, { expected: 404 });
    await stranger.request('GET', `/templates/${template.id}`, { expected: 404 });
    await stranger.request('GET', `/files/${originalFile.id}`, { expected: 404 });
    await stranger.request('POST', `/requests/${request.id}/share`, { expected: 404 });
  });
  await check('SMTP accepted by actual Mailpit and message exists', async () => {
    const delivery = (await office.request('POST', `/requests/${request.id}/notify`, { json: { channel: 'email' } })).data;
    assert.equal(delivery.status, 'accepted', 'Actual SMTP did not accept the message.');
    const message = await mailFor(clientEmail, `Documentos solicitados — ${organizationName}`);
    const text = message.Text ?? message.text ?? '';
    assert(text.includes('Synthetic integration collection') && text.includes('/p/'), 'Mailpit message does not contain the real request link.');
  });
  if (process.argv.includes('--eicar')) {
    await check('actual ClamAV rejects harmless EICAR signature', async () => {
      const eicar = Buffer.from('X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*');
      const testFile = syntheticPDF('antivirus-test', eicar);
      const denied = await upload(anonymous, token, request.items[0].id, 'synthetic-antivirus-test.pdf', 'application/pdf', testFile, 422);
      assert.equal(denied.error.code, 'malware_rejected');
      const current = (await office.request('GET', `/requests/${request.id}`)).data;
      assert.equal(current.items[0].files.length, 2, 'Rejected antivirus test was persisted.');
    });
  }
  await check('targeted correction preserves unaffected file and history', async () => {
    request = (await office.request('POST', `/requests/${request.id}/items/${request.items[0].id}/review`, { json: { action: 'correct', reason: 'Synthetic first PDF needs a replacement.', fileId: originalFile.id } })).data;
    assert.equal(request.status, 'correction_required');
    assert.equal(request.items[0].correctionFileId, originalFile.id);
    request = await upload(anonymous, token, request.items[0].id, 'synthetic-statement-v2.pdf', 'application/pdf', syntheticPDF('replacement'));
    assert.equal(request.items[0].files.length, 3);
    assert(request.items[0].files.find(file => file.id === originalFile.id).superseded);
    assert(!request.items[0].files.find(file => file.id === pngFile.id).superseded);
    assert.equal(request.items[0].files.filter(file => !file.superseded).length, 2);
    assert(request.items[0].files.every(file => file.scanStatus === 'clean'));
  });
  await check('approval completes request and closes additional uploads', async () => {
    request = (await office.request('POST', `/requests/${request.id}/items/${request.items[0].id}/review`, { json: { action: 'approve' } })).data;
    assert.equal(request.status, 'completed');
    assert.equal((await anonymous.request('GET', `/portal/${token}`)).data.request.status, 'completed');
    await upload(anonymous, token, request.items[0].id, 'must-not-upload.pdf', 'application/pdf', syntheticPDF('closed'), 409);
    await office.request('POST', `/requests/${request.id}/notify`, { json: { channel: 'email' }, expected: 409 });
  });
  await check('authorized ZIP contains only current synthetic documents', async () => {
    const { data, response } = await office.request('GET', `/requests/${request.id}/export`, { binary: true });
    assert.equal(response.headers.get('content-type'), 'application/zip');
    assert.equal(data.readUInt32LE(0), 0x04034b50, 'Export is not a ZIP.');
    assert(data.includes(Buffer.from('synthetic-statement-v2.pdf')) && data.includes(Buffer.from('synthetic-pixel.png')));
    assert(!data.includes(Buffer.from(`${originalFile.id.slice(0, 8)}-synthetic-statement.pdf`)), 'Superseded file leaked into current export.');
  });
  await check('sensitive request requires browser-specific actual email code', async () => {
    const created = (await office.request('POST', '/requests', { json: { clientIds: [client.id], templateId: template.id, title: 'Synthetic sensitive collection', competence: request.competence, dueDate: request.dueDate, sensitive: true } })).data;
    sensitive = created[0];
    sensitiveToken = (await office.request('POST', `/requests/${sensitive.id}/share`)).data.token;
    await portalBrowser.request('GET', `/portal/${sensitiveToken}`, { expected: 403 });
    await portalBrowser.request('POST', `/portal/${sensitiveToken}/code`);
    const email = await mailFor(clientEmail, 'Código de acesso DocPing');
    const code = /\b\d{6}\b/.exec(email.Text ?? email.text ?? '')?.[0];
    assert(code, 'Mailpit did not contain the actual code.');
    const verified = (await portalBrowser.request('POST', `/portal/${sensitiveToken}/verify`, { json: { code } })).data;
    assert.equal(verified.verified, true);
    assert.equal((await portalBrowser.request('GET', `/portal/${sensitiveToken}`)).data.request.id, sensitive.id);
    await anonymous.request('GET', `/portal/${sensitiveToken}`, { expected: 403 });
  });
  await check('public token cannot download another requests private file', async () => {
    await portalBrowser.request('GET', `/files/${pngFile.id}?token=${encodeURIComponent(sensitiveToken)}`, { expected: 404 });
  });
  await check('revoked link no longer grants portal or file access', async () => {
    await office.request('POST', `/requests/${request.id}/revoke`);
    await anonymous.request('GET', `/portal/${token}`, { expected: 410 });
    await anonymous.request('GET', `/files/${pngFile.id}?token=${encodeURIComponent(token)}`, { expected: 410 });
  });
  if (process.argv.includes('--mysql')) {
    await check('actual MySQL8 TestMySQLRoundTrip', async () => {
      const mysqlPassword = config('MYSQL_PASSWORD');
      assert(mysqlPassword, 'MYSQL_PASSWORD is required in the local .env for --mysql.');
      const mysqlUser = config('MYSQL_USER', 'docping');
      const database = config('MYSQL_DATABASE', 'docping');
      assert(/^[A-Za-z0-9_]+$/.test(mysqlUser) && /^[A-Za-z0-9_]+$/.test(database), 'Invalid local MySQL fixture configuration.');
      const dsn = `${mysqlUser}:${mysqlPassword}@tcp(127.0.0.1:3306)/${database}?parseTime=true&loc=UTC`;
      const result = spawnSync('go', ['test', '-run', '^TestMySQLRoundTrip$', '-count=1', '-v', './internal/persistence'], {
        cwd: path.join(root, 'apps/api'),
        env: { ...process.env, MYSQL_TEST_DSN: dsn, GOMODCACHE: config('GOMODCACHE', '/private/tmp/docping-go-mod'), GOCACHE: config('GOCACHE', '/private/tmp/docping-go-cache') },
        encoding: 'utf8', timeout: 120_000, maxBuffer: 2 << 20,
      });
      // Never echo child diagnostics: a driver/configuration error could contain
      // sensitive configuration. Only a real PASS, never SKIP, counts here.
      assert.equal(result.status, 0, 'Real MySQL integration failed; child diagnostics are suppressed to protect local configuration.');
      assert(result.stdout.includes('--- PASS: TestMySQLRoundTrip') && !result.stdout.includes('--- SKIP:'), 'MySQL integration did not execute.');
    });
  }
  const report = { status: 'passed', completedAt: new Date().toISOString(), elapsedMs: Date.now() - started, checks, notExecuted: [!process.argv.includes('--eicar') && 'EICAR rejection (use --eicar)', !process.argv.includes('--mysql') && 'Direct MySQL integration (use --mysql)', 'Browser/Playwright usability and responsive tests'].filter(Boolean) };
  const reportPath = path.join(root, 'test-results', 'integration-smoke.json');
  await mkdir(path.dirname(reportPath), { recursive: true });
  await writeFile(reportPath, `${JSON.stringify(report, null, 2)}\n`, { mode: 0o600 });
  console.log(`Integration passed: ${checks.length} checks. Sanitized report: test-results/integration-smoke.json`);
} catch (error) {
  // Assertion values and stack traces are intentionally not printed: they can
  // contain tokens/cookies even though the fixture itself is synthetic.
  const message = error instanceof Error ? error.message : 'Unknown integration failure.';
  const safe = message.replace(/[a-f0-9]{64}/g, '[redacted]').replace(/https?:\/\/[^\s]+\/p\/[^\s]+/g, '[public link redacted]');
  console.error(`FAIL ${stage}: ${safe}`);
  const reportPath = path.join(root, 'test-results', 'integration-smoke.json');
  await mkdir(path.dirname(reportPath), { recursive: true });
  await writeFile(reportPath, `${JSON.stringify({ status: 'failed', completedAt: new Date().toISOString(), elapsedMs: Date.now() - started, stage, error: safe, checks }, null, 2)}\n`, { mode: 0o600 });
  process.exitCode = 1;
}
