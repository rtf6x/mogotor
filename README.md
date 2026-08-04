# Mogotor

Small server analytics dashboard for Linux hosts.

Shows CPU, memory, disk, network, and load averages with 24-hour charts, plus PM2 processes, Docker containers (including Redis/Mongo/RabbitMQ), systemd host services, and MongoDB/Redis status.

## Quick start

```bash
make build
./bin/mogotor
```

Open http://localhost:8188

Listen address defaults to `:8188`. Override with `MOGOTOR_ADDR`, for example `:8080`.

History is stored in Redis (database 4 by default; on ci.rootfox.cc Redis runs in Docker). Set `MOGOTOR_REDIS_ADDR` or `REDIS_ADDR` (default `127.0.0.1:63719`), `REDIS_PASSWORD`, and optionally `MOGOTOR_REDIS_DB`.

RabbitMQ Management API (optional panel): `MOGOTOR_RABBIT_URL` (default `http://127.0.0.1:15672/rabbit`), `MOGOTOR_RABBIT_USER` (default `rootfox`), `MOGOTOR_RABBIT_PASSWORD`.

Systemd service list watches host units only (nginx, docker, dplo, fail2ban). Redis, MongoDB, and OpenVPN are monitored via their own panels / Docker, not as host systemd units.

## Deploy

```bash
sudo ./deploy/install.sh
```

Installs a systemd service, binary to `/opt/mogotor`, and history data in Redis. Put `REDIS_PASSWORD` in `/etc/mogotor/env`.

For Docker stats, the service user needs access to the Docker socket (group membership or `sudo docker`).

## API

- `GET /api/snapshot` - latest metrics snapshot
- `GET /api/history` - 24h system history
- `GET /api/health` - service health

## Security

No authentication yet. Do not expose publicly without a reverse proxy or firewall in front.
