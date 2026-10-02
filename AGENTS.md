# Trabajar en GOWA

GOWA es un servidor de API de WhatsApp en Go que usa whatsmeow, Fiber v3 y SQLite. El
módulo Go y el directorio de ejecución local son `src/`; todo lo que hay dentro es un
único módulo (`github.com/aldinokemal/go-whatsapp-web-multidevice`).
`go run . rest` sirve REST, el dashboard en `/` y MCP en `{APP_BASE_PATH}/mcp`. Los
tres transportes comparten el mismo device manager y los mismos usecases.

Este archivo describe la estructura del proyecto. Las diferencias de este fork
respecto a upstream (remotos, CI, despliegue) están en
[docs/HLCODES.md](docs/HLCODES.md) y [docs/DOKPLOY.md](docs/DOKPLOY.md).

## Puntos de entrada por tarea

Lee el `AGENTS.md` con alcance propio del área que toques; ahí están los contratos.

- DTOs e interfaces: [domains](src/domains/AGENTS.md). Orquestación de negocio:
  [usecases](src/usecase/AGENTS.md). Reglas de entrada: [validation](src/validations/AGENTS.md).
- Transporte REST, MCP y websocket: [UI adapters](src/ui/AGENTS.md). Para la
  autenticación de MCP, usa [la guía de OAuth](docs/mcp-oauth.md).
- Ciclo de vida de dispositivos, eventos, JIDs y presencia: [infraestructura de WhatsApp](src/infrastructure/whatsapp/AGENTS.md).
  Para cambios en el payload de webhook, consulta [el contrato del payload](docs/webhook-payload.md).
- Consultas SQL, migraciones y limpieza: [chat storage](src/infrastructure/chatstorage/AGENTS.md).
- Enrutado de Chatwoot, sync en vivo e historial: [infraestructura de Chatwoot](src/infrastructure/chatwoot/AGENTS.md).
  Cambios de setup o modo de enrutado: [configuración](docs/chatwoot.md).
  Cambios directos en Postgres: [guía con alcance](src/infrastructure/chatwoot/pgimport/AGENTS.md).
- Arranque, configuración y orden de rutas: `src/cmd/root.go`, `src/cmd/rest.go`,
  `src/cmd/helpers.go`, `src/cmd/mcp_oauth.go`, `src/config/settings.go` y
  `src/.env.example`. La config son globals a nivel de paquete en `config`; los flags
  sobrescriben a Viper, que lee `.env`, que a su vez sobrescribe los defaults del código.
- **El dashboard no está en este repo.** No existe `src/views/`. `gowa-ui` es un
  proyecto aparte publicado como un único asset HTML que se descarga al arrancar
  mediante `src/infrastructure/uiasset/`, se cachea en `storages/ui/` y se sirve en
  `/`. Los cambios de comportamiento del dashboard van en `aldinokemal/gowa-ui`; aquí
  solo vive el wiring de descarga, caché y ruta.
- Packaging: `docker/golang.Dockerfile`, `docker/entrypoint.sh` y `.github/workflows/`.
  Las versiones del toolchain se leen de esos archivos y de `src/go.mod`. `AppVersion`
  es una constante en el código, en `src/config/settings.go`; los builds de release no
  la inyectan (solo ldflags `-s -w`). `release.yml` corre únicamente con un tag
  `v[0-9]+.[0-9]+.[0-9]+` pusheado y escribe su config de GoReleaser en `/tmp`; no hay
  `.goreleaser.yml` commiteado. Las imágenes Docker se publican con tags o de forma manual.
- Publicar un release estable: usa [new-release](.agents/skills/new-release/SKILL.md)
  solo cuando se invoque explícitamente. Para las notas de un release existente, usa
  [update-release-note](.agents/skills/update-release-note/SKILL.md).

## Contratos entre capas

- Mantén el parseo de transporte en `ui/`, la orquestación en `usecase/` y los
  DTOs/interfaces en `domains/`. Preserva los campos JSON/form que usan REST, MCP y los
  clientes de navegador.
- El acceso de usuario a chats y mensajes debe llevar el alcance del dispositivo. Tras
  el login, el almacenamiento usa `client.Store.ID.ToNonAD().String()`, no el alias
  visible del dispositivo. `scheduled_sends` es la excepción deliberada: se indexa por
  el alias del registro para que el worker pueda resolver clientes con
  `DeviceManager.GetDevice`.
- Los cambios en una interface de repositorio deben llegar a las tres capas:
  `src/domains/chatstorage/interfaces.go`, `src/infrastructure/chatstorage/sqlite_repository.go`
  y `src/infrastructure/whatsapp/chatstorage_wrapper.go`.
- Los tests que mutan config, globals de paquete o estado del scheduler se mantienen
  seriales y restauran ese estado. Reusa los helpers y stubs existentes en el paquete.

## Comandos y validación

Ejecuta los comandos de Go desde `src/`. Elige las comprobaciones según el comportamiento
afectado:

```sh
go test ./path/to/affected/package/...   # un paquete o subárbol
go test ./...                            # contratos compartidos, arranque, cambios amplios
go vet ./...
go build -o whatsapp .
```

No hay config de lint ni Makefile; `gofmt` es el único formateador.
Los tests son 108 archivos `_test.go` colocados junto al código, con SQLite real en
memoria o temporal, `go-sqlmock` para Postgres y `app.Test` de Fiber: no hacen falta
servicios externos. Para cambios solo de Markdown, revisa el diff y los enlaces en vez
de correr tests. Reporta qué comprobaciones se ejecutaron de verdad y qué quedó sin
verificar.

Los builds por defecto de SQLite usan CGO (`src/pkg/sqlite/sqlite_cgo.go`). El tag
`purego` selecciona `modernc.org/sqlite` (`sqlite_purego.go`) para cross-compilar sin
CGO. Ejercita ambas variantes al cambiar SQL dependiente del driver. Corre `go mod
tidy` al cambiar dependencias.

Las rutas de runtime son relativas al directorio del proceso: una ejecución directa
usa `src/storages` y `src/statics`, mientras que Docker Compose monta los directorios de
la raíz `storages/` y `statics/` en `/app`. Mantén ambos excluidos del hot reload en
`src/.air.toml`. El entrypoint hace chown de esos volúmenes antes de bajar a
`gowauser` (uid 20001, gid 20000), así que debe seguir siendo root en el contenedor.

Mantén `.env`, bases SQLite, datos de sesión, códigos QR, medios generados y volcados de
historial fuera de los commits, y preserva los cambios no relacionados del working tree.
Los tests locales con bases temporales y clientes falsos no requieren aprobación;
ejecutar la app contra sesiones guardadas reconecta dispositivos reales de WhatsApp, así
que mantenlo dentro del alcance autorizado por el usuario.