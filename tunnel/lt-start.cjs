/* Puru Web -> localtunnel (nama bisa dipilih). Jalan tanpa CLI (CLI crash di Android). */
const fs = require('fs');
const path = require('path');
const os = require('os');

const PORT = Number(process.env.LT_PORT || 18099);
const SUBS = (process.env.LT_SUB || 'puru-ai,puruweb,puru,puruai').split(',').map(s => s.trim()).filter(Boolean);
const DIR = __dirname;
const URLFILE = path.join(DIR, 'lt.url');
const PIDFILE = path.join(DIR, 'lt.pid');
const LOGFILE = path.join(DIR, 'lt.log');

const log = (...a) => { try { fs.appendFileSync(LOGFILE, new Date().toISOString() + ' ' + a.join(' ') + '\n'); } catch (e) {} };

// pidfile: dipakai tunnel/lt-pid.sh biar tidak perlu scan /proc
// (scan /proc bisa "menemukan" shell pemanggil sendiri -> shell-nya bunuh diri)
try { fs.writeFileSync(PIDFILE, String(process.pid)); } catch (e) {}
const cleanPid = () => { try { if (Number(fs.readFileSync(PIDFILE, 'utf8')) === process.pid) fs.unlinkSync(PIDFILE); } catch (e) {} };
process.on('exit', cleanPid);
['SIGTERM', 'SIGINT', 'SIGHUP'].forEach(sig => { try { process.on(sig, () => { cleanPid(); process.exit(0); }); } catch (e) {} });

let localtunnel;
const tries = [
  'localtunnel',
  '/data/data/com.termux/files/usr/lib/node_modules/localtunnel',
  path.join(os.homedir(), '.npm/_npx/75ac80b86e83d4a2/node_modules/localtunnel'),
];
for (const p of tries) { try { localtunnel = require(p); break; } catch (e) {} }
if (!localtunnel) { log('FATAL modul localtunnel tidak ketemu'); process.exit(1); }

(async () => {
  for (const sub of SUBS) {
    try {
      const t = await localtunnel({ port: PORT, subdomain: sub, local_host: '127.0.0.1' });
      const host = new URL(t.url).hostname;
      if (host !== sub + '.loca.lt') log('WARN subdomain ' + sub + ' tidak dapat -> ' + host);
      log('UP url=' + t.url + ' (diminta: ' + sub + ')');
      fs.writeFileSync(URLFILE, t.url + '\n');
      t.on('close', () => { log('CLOSED'); try { fs.unlinkSync(URLFILE); } catch (e) {} });
      t.on('error', e => log('ERROR ' + (e && e.message)));
      const bye = () => { try { t.close(); } catch (e) {} cleanPid(); process.exit(0); };
      process.on('SIGTERM', bye); process.on('SIGINT', bye);
      setInterval(() => {}, 1 << 30);
      return;
    } catch (e) { log('GAGAL sub ' + sub + ': ' + (e && e.message)); }
  }
  log('FATAL semua subdomain gagal');
  process.exit(1);
})();
