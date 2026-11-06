# Deploy de teste na AWS (EC2 + Docker Compose)

Tudo em uma única instância: Postgres, API e Caddy (HTTPS automático + frontend). Só para teste, sem backup gerenciado.
Arquivos: `deploy/` (Dockerfiles, Caddyfile, compose, `.env.example`).

## 1. Conta e instância (console da AWS)
1. Região: `us-east-1` ou `sa-east-1` (São Paulo, mais perto, um pouco mais cara).
2. **EC2 → Launch instance**
   - AMI: Ubuntu Server 24.04 LTS
   - Tipo: `t3.small` (2 GB). Com `t3.micro` (1 GB) o build do frontend pode falhar por falta de memória; se usar, crie swap (passo 3).
   - Key pair: crie e guarde o `.pem`.
   - Disco: 20 GB gp3.
   - Security group: SSH (22) **só do seu IP**, HTTP (80) e HTTPS (443) de qualquer lugar. O Postgres NÃO é aberto.
3. **Elastic IP**: aloque e associe à instância (o IP não muda se reiniciar).
4. Seu endereço sem domínio: IP `3.90.12.34` → `3-90-12-34.sslip.io` (o sslip.io resolve para o IP e o Caddy obtém o certificado).
5. **Billing → Budgets**: crie um alerta de US$ 5.

## 2. Docker no servidor
```bash
ssh -i chave.pem ubuntu@SEU_IP
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker ubuntu && exit   # entre de novo
```

## 3. (só t3.micro) swap
```bash
sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile
```

## 4. Código e configuração
```bash
git clone <URL_DO_REPOSITORIO> casal && cd casal/deploy   # a branch que quer testar
cp .env.example .env && nano .env      # senhas novas (openssl rand -hex 24) e SITE_ADDRESS
docker compose build                   # alguns minutos
docker compose run --rm --entrypoint vapid api   # cole as chaves no .env
docker compose up -d
docker compose logs -f api web
```
Abra `https://SEU-IP-COM-HIFENS.sslip.io`.

## 5. Atualizar para a última versão
```bash
cd ~/casal && git pull && cd deploy && docker compose up -d --build
```
As migrações rodam sozinhas (serviço `migrate`) antes da API subir.

## Notas
- `db-init.sh` cria o papel `casal_app` só na **primeira** subida do volume. Se trocar `APP_DB_PASSWORD` depois, rode `ALTER ROLE casal_app PASSWORD '...'` no banco.
- Zerar tudo: `docker compose down -v` (apaga o banco).
- Backup manual: `docker compose exec db pg_dump -U casal casal > backup.sql`.
- Parar de pagar: **Terminate** a instância e **libere o Elastic IP** (IP parado cobra).
