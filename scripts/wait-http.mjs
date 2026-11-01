// Espera uma URL responder (2xx). Uso: node scripts/wait-http.mjs <url> [segundos]
// Se não responder a tempo, avisa e sai com sucesso: quem chamou segue em frente mesmo assim.
const [url, secs = '60'] = process.argv.slice(2)
if (!url) {
  console.error('uso: node scripts/wait-http.mjs <url> [segundos]')
  process.exit(2)
}
const deadline = Date.now() + Number(secs) * 1000
process.stdout.write(`Aguardando ${url} `)
while (Date.now() < deadline) {
  try {
    const res = await fetch(url, { signal: AbortSignal.timeout(2000) })
    if (res.ok) {
      console.log('✔')
      process.exit(0)
    }
  } catch {
    // ainda não subiu
  }
  process.stdout.write('.')
  await new Promise((r) => setTimeout(r, 500))
}
console.log(`\n⚠ ${url} não respondeu em ${secs}s; seguindo mesmo assim.`)
