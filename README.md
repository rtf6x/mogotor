# Mogotor

Small server analytics dashboard for Linux hosts.

Shows CPU, memory, disk, network, and load averages with 24-hour charts, plus PM2 processes, Docker containers, systemd services, and MongoDB status.

## Quick start

```bash
make build
./bin/mogotor
```

Open http://localhost:8188

Listen address defaults to `:8188`. Override with `MOGOTOR_ADDR`, for example `:8080`.

## Deploy

```bash
sudo ./deploy/install.sh
```

Installs a systemd service, binary to `/opt/mogotor`, and history data to `/var/lib/mogotor`.

For Docker stats, the service user needs access to the Docker socket (group membership or `sudo docker`).

## API

- `GET /api/snapshot` - latest metrics snapshot
- `GET /api/history` - 24h system history
- `GET /api/health` - service health

## Security

No authentication yet. Do not expose publicly without a reverse proxy or firewall in front.
