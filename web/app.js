const out = (m) => { document.getElementById('log').textContent += m + '\n'; };
const api = (path, opts = {}) =>
  fetch(path, { credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, ...opts });

// Mostra o status e, quando houver, a mensagem de erro do servidor.
async function report(label, res) {
  let msg = '';
  if (!res.ok) {
    try { msg = ' (' + (await res.json()).error + ')'; } catch (_) {}
  }
  out(label + ': ' + res.status + msg);
}

function urlBase64ToUint8Array(base64) {
  const pad = '='.repeat((4 - (base64.length % 4)) % 4);
  const raw = atob((base64 + pad).replace(/-/g, '+').replace(/_/g, '/'));
  return Uint8Array.from([...raw].map((c) => c.charCodeAt(0)));
}

document.getElementById('login').addEventListener('submit', async (e) => {
  e.preventDefault();
  const res = await api('/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({
      email: document.getElementById('email').value,
      password: document.getElementById('password').value,
    }),
  });
  await report('login', res);
});

document.getElementById('enable').addEventListener('click', async () => {
  if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
    out('Sem suporte a push. No iPhone, instale o app na tela inicial.');
    return;
  }
  try {
    const reg = await navigator.serviceWorker.register('/sw.js');
    await navigator.serviceWorker.ready;
    if ((await Notification.requestPermission()) !== 'granted') {
      out('permissão negada');
      return;
    }
    const { public_key } = await (await api('/v1/push/vapid-key')).json();
    const sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(public_key),
    });
    const res = await api('/v1/push/subscriptions', { method: 'POST', body: JSON.stringify(sub.toJSON()) });
    await report('assinatura', res);
  } catch (err) {
    out('erro: ' + err.message);
  }
});

document.getElementById('test').addEventListener('click', async () => {
  const res = await api('/v1/push/test', { method: 'POST' });
  await report('teste', res);
});
