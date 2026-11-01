// Prepara o banco para o desenvolvimento: sobe o Postgres, garante o papel do app e aplica as migrações.
// É o que o F5 do VS Code roda antes de iniciar o backend. Pode rodar à mão: node scripts/dev-db.mjs
// Não depende de make, então funciona igual no PowerShell, no Git Bash e no Linux.
import { spawnSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')

function fail(msg) {
  console.error(`\n✖ ${msg}`)
  process.exit(1)
}

function loadEnv(file) {
  if (!existsSync(file)) fail('Arquivo .env não encontrado. Rode: cp .env.example .env (e ajuste as senhas).')
  const env = {}
  for (const line of readFileSync(file, 'utf8').split(/\r?\n/)) {
    const m = line.match(/^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*?)\s*$/)
    if (m && !line.trim().startsWith('#')) env[m[1]] = m[2].replace(/^(['"])(.*)\1$/, '$2')
  }
  return env
}

function run(title, cmd, args, opts = {}) {
  console.log(`\n▶ ${title}`)
  const r = spawnSync(cmd, args, { cwd: root, stdio: 'inherit', ...opts })
  if (r.error?.code === 'ENOENT') fail(`"${cmd}" não encontrado no PATH.${opts.hint ? ' ' + opts.hint : ''}`)
  if (r.status !== 0) fail(`Falhou: ${title}`)
}

const env = loadEnv(join(root, '.env'))
const need = (k) => env[k] || fail(`Defina ${k} no .env (veja .env.example).`)

// 1. Postgres em Docker, esperando ficar saudável
run('Subindo o Postgres (Docker)', 'docker', ['compose', 'up', '-d', '--wait', 'db'], {
  env: { ...process.env, ...env },
  hint: 'Instale/abra o Docker Desktop.',
})

// 2. Papel limitado do app (o RLS vale para ele). Só se APP_DB_PASSWORD estiver no .env.
if (env.APP_DB_PASSWORD) {
  const pw = env.APP_DB_PASSWORD.replaceAll("'", "''")
  const sql = [
    `DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'casal_app') THEN CREATE ROLE casal_app LOGIN PASSWORD '${pw}'; END IF; END $$;`,
    `GRANT CONNECT ON DATABASE ${need('DB_NAME')} TO casal_app;`,
    'GRANT USAGE ON SCHEMA public TO casal_app;',
  ]
  run('Garantindo o papel casal_app', 'docker', [
    'compose', 'exec', '-T', 'db', 'psql', '-U', need('DB_USER'), '-d', need('DB_NAME'), '-v', 'ON_ERROR_STOP=1',
    ...sql.flatMap((s) => ['-c', s]),
  ], { env: { ...process.env, ...env } })
} else {
  console.log('\n• APP_DB_PASSWORD ausente no .env: o papel casal_app não foi tocado (o app usará o DATABASE_URL como está).')
}

// 3. Migrações, com o dono do banco
const url = env.MIGRATE_DATABASE_URL || need('DATABASE_URL')
run('Aplicando as migrações', 'goose', ['-dir', 'migrations', 'postgres', url, 'up'], {
  hint: 'Instale: go install github.com/pressly/goose/v3/cmd/goose@latest',
})

console.log('\n✔ Banco pronto.')

if (!/^postgres(ql)?:\/\/casal_app[:@]/.test(need('DATABASE_URL'))) {
  console.log('⚠ DATABASE_URL não usa o papel casal_app: o app conecta como dono do banco e o RLS não protege nada.')
  console.log('  Veja .env.example (APP_DB_PASSWORD, DATABASE_URL e MIGRATE_DATABASE_URL).')
}
