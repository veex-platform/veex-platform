# VEEX Registry & Platform ️

The cloud brain of the ecosystem. Manage artifact lifecycles and fleet observability.

## Components:
- **Registry API**: Secure storage for `.vex` binaries with versioning and Ed25519 signatures.
- **Observability Hub**: Ingestion of industrial logs and metrics from the Runtime.
- **Provisioning Service**: "Zero-Touch" delivery via OTA for devices in the field.

## Endpoints:
- `GET /api/v1/templates`: Fetch dynamic industrial logic templates.
- `POST /api/v1/registry/upload`: Artifact publication.
- `GET /api/v1/registry/download`: OTA binary download.
- `GET /api/v1/ota/devices`: List all registered industrial devices.
- `GET /api/v1/ota/fleets`: Retrieve configured device groups.
- `POST /api/v1/ota/campaigns`: Initiate new update rollouts.
- `POST /api/v1/observability/ingest`: Receive vital health signals.

## On-Premise Distribution
When deploying VEEX Platform locally (via Docker), you must ensure that the devices have network access to the container.

### Firmware Configuration
**Important**: The `veex-runtime` (firmware) must be flashed with the correct IP/URL of your local Registry instance.
- Default: `http://localhost:80` (suitable for Unified Gateway local setups).
- On-Premise: Point to your server's static IP (e.g., `http://192.168.1.50:80`).

## Database Configuration
The platform supports both SQLite (local/edge) and PostgreSQL (production).

| Environment Variable | Description | Default |
| :--- | :--- | :--- |
| `DB_TYPE` | Type of database: `sqlite` or `postgres` | `sqlite` |
| `DATABASE_URL` | Connection string for PostgreSQL | - |
| `DATA_DIR` | Path to SQLite and registry data | `data` |
| `STORAGE_DIR` | Path to artifact binaries | `storage` |

**PostgreSQL Example:**
`DB_TYPE=postgres DATABASE_URL=postgres://user:pass@localhost:5432/veex?sslmode=disable`

---
[Official Site](https://github.com/veex-platform) | [API Docs](https://github.com/veex-platform/veex-docs/technical-reference)




